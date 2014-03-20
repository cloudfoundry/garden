#include <time.h>
#include <assert.h>
#include <linux/limits.h>
#include <pthread.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>
#include <errno.h>

#include "child.h"
#include "dlog.h"
#include "muxer.h"
#include "status_writer.h"
#include "util.h"

static pthread_mutex_t link_wait_waiting = PTHREAD_MUTEX_INITIALIZER;
static pthread_cond_t  link_wait_done    = PTHREAD_COND_INITIALIZER;

typedef struct link_wait_s {
  muxer_t         **muxers;
  barrier_t       *barrier;
} link_wait_t;

void *wait_for_link(void *data) {
  assert(NULL != data);

  int oldtype;
  pthread_setcanceltype(PTHREAD_CANCEL_ASYNCHRONOUS, &oldtype);

  /* Wait for clients on stdout, stderr, and status */

  muxer_t **muxers = ((link_wait_t *)data)->muxers;
  barrier_t *barrier = ((link_wait_t *)data)->barrier;

  int ii=0;
  for (ii = 0; ii < 2; ++ii) {
    muxer_wait_for_client(muxers[ii]);
  }
  barrier_wait(barrier);


  pthread_cond_signal(&link_wait_done);
  return NULL;
}

static void *run_muxer(void *data) {
  assert(NULL != data);

  muxer_run((muxer_t *) data);

  return NULL;
}

static void *run_status_writer(void *data) {
  assert(NULL != data);

  status_writer_run((status_writer_t *) data);

  return NULL;
}

void usage(char * program_name, int exit_status) {
  fprintf(stderr, "Usage: %s [-t <timeout>] <socket directory> <cmd...>\n", program_name);
  exit(exit_status);
}

int main(int argc, char *argv[]) {
  int              backlog          = 10;
  muxer_t         *muxers[2]        = {NULL, NULL};
  status_writer_t *sw               = NULL;
  child_t         *child            = NULL;
  int              child_status     = -1;
  int              ring_buffer_size = 65535;
  int              fds[3]           = {-1, -1, -1};
  int              ii               = 0, exit_status = 0, nwritten = 0;
  pthread_t        sw_thread, muxer_threads[2];
  char             socket_paths[3][PATH_MAX + 1];
  char             *socket_names[3] = { "stdout.sock", "stderr.sock", "status.sock" };
  barrier_t        *barrier = NULL;
  int              link_wait_timeout = 5;

  int              optind = 1;

  if (strcmp(argv[1],"-t") == 0)  {
    link_wait_timeout = atoi(argv[2]);
    optind = 3;
  }

  if (argc <= (optind + 2)) {
    usage(argv[0], EXIT_FAILURE);
  }

  /* Setup listeners on domain sockets */
  for (ii = 0; ii < 3; ++ii) {
    memset(socket_paths[ii], 0, sizeof(socket_paths[ii]));
    nwritten = snprintf(socket_paths[ii], sizeof(socket_paths[ii]),
                        "%s/%s", argv[optind], socket_names[ii]);
    if (nwritten >= sizeof(socket_paths[ii])) {
      fprintf(stderr, "Socket path too long\n");
      exit_status = 1;
      goto cleanup;
    }

    fds[ii] = create_unix_domain_listener(socket_paths[ii], backlog);

    DLOG("created listener, path=%s fd=%d", socket_paths[ii], fds[ii]);

    if (-1 == fds[ii]) {
      perrorf("Failed creating socket at %s:", socket_paths[ii]);
      exit_status = 1;
      goto cleanup;
    }

    set_cloexec(fds[ii]);
  }

  /*
   * Make sure iomux-spawn runs in an isolated process group such that
   * it is not affected by signals sent to its parent's process group.
   */
  setsid();

  child = child_create(argv + optind + 1, argc - optind - 1);

  printf("child_pid=%d\n", child->pid);
  fflush(stdout);

  /* Muxers for stdout/stderr */
  muxers[0] = muxer_alloc(fds[0], child->stdout[0], ring_buffer_size);
  muxers[1] = muxer_alloc(fds[1], child->stderr[0], ring_buffer_size);
  for (ii = 0; ii < 2; ++ii) {
    if (pthread_create(&muxer_threads[ii], NULL, run_muxer, muxers[ii])) {
      perrorf("Failed creating muxer thread:");
      exit_status = 1;
      goto cleanup;
    }

    DLOG("created muxer thread for socket=%s", socket_paths[ii]);
  }

  /* Status writer */
  barrier = barrier_alloc();
  sw = status_writer_alloc(fds[2], barrier);
  if (pthread_create(&sw_thread, NULL, run_status_writer, sw)) {
    perrorf("Failed creating muxer thread:");
    exit_status = 1;
    goto cleanup;
  }

  link_wait_t *link_wait = calloc(1, sizeof(link_wait_t));

  link_wait->muxers = muxers;
  link_wait->barrier = barrier;

  pthread_mutex_lock(&link_wait_waiting);

  struct timespec abs_time;
  pthread_t tid;
  int err;

  clock_gettime(CLOCK_REALTIME, &abs_time);
  abs_time.tv_sec += link_wait_timeout;

  pthread_create(&tid, NULL, wait_for_link, (void *) link_wait);

  err = pthread_cond_timedwait(&link_wait_done, &link_wait_waiting, &abs_time);

  if (err) {
    exit_status = err;
    goto cleanup;
  }

  pthread_mutex_unlock(&link_wait_waiting);
  pthread_join(tid, NULL);

  child_continue(child);

  printf("child active\n");
  fflush(stdout);

  if (-1 == waitpid(child->pid, &child_status, 0)) {
    perrorf("Waitpid for child failed: ");
    exit_status = 1;
    goto cleanup;
  }

  DLOG("child exited, status = %d", WEXITSTATUS(child_status));

  /* Wait for status writer */
  status_writer_finish(sw, child_status);
  pthread_join(sw_thread, NULL);

  /* Wait for muxers */
  for (ii = 0; ii < 2; ++ii) {
    muxer_stop(muxers[ii]);
    pthread_join(muxer_threads[ii], NULL);
  }

  DLOG("all done, cleaning up and exiting");

cleanup:
  if (NULL != child) {
    child_free(child);
  }

  if (NULL != barrier) {
    barrier_free(barrier);
  }

  if (NULL != sw) {
    status_writer_free(sw);
  }

  for (ii = 0; ii < 2; ++ii) {
    if (NULL != muxers[ii]) {
      muxer_free(muxers[ii]);
    }
  }

  /* Close accept sockets and clean up paths */
  for (ii = 0; ii < 3; ++ii) {
    if (-1 != fds[ii]) {
      close(fds[ii]);
      unlink(socket_paths[ii]);
    }
  }

  return exit_status;
}
