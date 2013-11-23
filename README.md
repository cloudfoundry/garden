```
                                                 ,-.
                                                  ) \
                                              .--'   |
                                             /       /
                                             |_______|
                                            (  O   O  )
                                             {'-(_)-'}
                                           .-{   ^   }-.
                                          /   '.___.'   \
                                         /  |    o    |  \
                                         |__|    o    |__|
                                         (((\_________/)))
                                             \___|___/
                                        jgs.--' | | '--.
                                           \__._| |_.__/
```

Warden in Go, because why not.

* [Tracker](https://www.pivotaltracker.com/s/projects/962374)
* [Travis](https://travis-ci.org/vito/garden)
* [Warden](https://github.com/cloudfoundry/warden)

# Running

For development, you can just spin up the Vagrant VM and run the server
locally, pointing at its host:

```bash
vagrant up
./bin/run-garden-remote-linux
```

The paths provided are actually the remote paths, so they don't have to exist
locally. They're created in the VM with `vagrant up`.

Normally (in production) Garden would be running on the same machine. In this
case just don't pass -remoteHost.
