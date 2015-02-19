package connection_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/ghttp"

	"github.com/cloudfoundry-incubator/garden"
	. "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/cloudfoundry-incubator/garden/transport"
)

var _ = Describe("Connection", func() {
	var (
		connection     Connection
		resourceLimits garden.ResourceLimits
		server         *ghttp.Server
	)

	BeforeEach(func() {
		server = ghttp.NewServer()
	})

	JustBeforeEach(func() {
		connection = New("tcp", server.HTTPTestServer.Listener.Addr().String())
	})

	BeforeEach(func() {
		rlimits := &garden.ResourceLimits{
			As:         proto.Uint64(1),
			Core:       proto.Uint64(2),
			Cpu:        proto.Uint64(4),
			Data:       proto.Uint64(5),
			Fsize:      proto.Uint64(6),
			Locks:      proto.Uint64(7),
			Memlock:    proto.Uint64(8),
			Msgqueue:   proto.Uint64(9),
			Nice:       proto.Uint64(10),
			Nofile:     proto.Uint64(11),
			Nproc:      proto.Uint64(12),
			Rss:        proto.Uint64(13),
			Rtprio:     proto.Uint64(14),
			Sigpending: proto.Uint64(15),
			Stack:      proto.Uint64(16),
		}

		resourceLimits = garden.ResourceLimits{
			As:         rlimits.As,
			Core:       rlimits.Core,
			Cpu:        rlimits.Cpu,
			Data:       rlimits.Data,
			Fsize:      rlimits.Fsize,
			Locks:      rlimits.Locks,
			Memlock:    rlimits.Memlock,
			Msgqueue:   rlimits.Msgqueue,
			Nice:       rlimits.Nice,
			Nofile:     rlimits.Nofile,
			Nproc:      rlimits.Nproc,
			Rss:        rlimits.Rss,
			Rtprio:     rlimits.Rtprio,
			Sigpending: rlimits.Sigpending,
			Stack:      rlimits.Stack,
		}
	})

	Describe("Ping", func() {
		Context("when the response is successful", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(200, "{}"),
					),
				)
			})

			It("should ping the server", func() {
				err := connection.Ping()
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when the request fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/ping"),
						ghttp.RespondWith(500, ""),
					),
				)
			})

			It("should return an error", func() {
				err := connection.Ping()
				Ω(err).Should(HaveOccurred())
			})
		})
	})

	Describe("Getting capacity", func() {
		Context("when the response is successful", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/capacity"),
						ghttp.RespondWith(200, marshalProto(&garden.Capacity{
							MemoryInBytes: 1111,
							DiskInBytes:   2222,
							MaxContainers: 42,
						}))))
			})

			It("should return the server's capacity", func() {
				capacity, err := connection.Capacity()
				Ω(err).ShouldNot(HaveOccurred())

				Ω(capacity.MemoryInBytes).Should(BeNumerically("==", 1111))
				Ω(capacity.DiskInBytes).Should(BeNumerically("==", 2222))
				Ω(capacity.MaxContainers).Should(BeNumerically("==", 42))
			})
		})

		Context("when the request fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/capacity"),
						ghttp.RespondWith(500, "")))
			})

			It("should return an error", func() {
				_, err := connection.Capacity()
				Ω(err).Should(HaveOccurred())
			})
		})
	})

	Describe("Creating", func() {
		var spec garden.ContainerSpec

		JustBeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/containers"),
					verifyRequestBody(&spec, &garden.ContainerSpec{}),
					ghttp.RespondWith(200, marshalProto(&struct{ Handle string }{"foohandle"}))))
		})

		Context("with an empty ContainerSpec", func() {
			BeforeEach(func() {
				spec = garden.ContainerSpec{}
			})

			It("sends the ContainerSpec over the connection as JSON", func() {
				handle, err := connection.Create(spec)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(handle).Should(Equal("foohandle"))
			})
		})

		Context("with a fully specified ContainerSpec", func() {
			BeforeEach(func() {
				spec = garden.ContainerSpec{
					Handle:     "some-handle",
					GraceTime:  10 * time.Second,
					RootFSPath: "some-rootfs-path",
					Network:    "some-network",
					BindMounts: []garden.BindMount{
						{
							SrcPath: "/src-a",
							DstPath: "/dst-a",
							Mode:    garden.BindMountModeRO,
							Origin:  garden.BindMountOriginHost,
						},
						{
							SrcPath: "/src-b",
							DstPath: "/dst-b",
							Mode:    garden.BindMountModeRW,
							Origin:  garden.BindMountOriginContainer,
						},
					},
					Properties: map[string]string{
						"foo": "bar",
					},
					Env: []string{"env1=env1Value1"},
				}
			})

			It("sends the ContainerSpec over the connection as JSON", func() {
				handle, err := connection.Create(spec)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(handle).Should(Equal("foohandle"))
			})
		})
	})

	Describe("Destroying", func() {
		Context("when destroying succeeds", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/foo"),
						ghttp.RespondWith(200, "{}")))
			})

			It("should stop the container", func() {
				err := connection.Destroy("foo")
				Ω(err).ShouldNot(HaveOccurred())
			})
		})

		Context("when destroying fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("DELETE", "/containers/foo"),
						ghttp.RespondWith(423, "some error")))
			})

			It("return an appropriate error with the code and message", func() {
				err := connection.Destroy("foo")
				Ω(err).Should(MatchError(Error{423, "some error"}))
			})
		})
	})

	Describe("Stopping", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("PUT", "/containers/foo/stop"),
					verifyRequestBody(map[string]interface{}{
						"kill": true,
					}, make(map[string]interface{})),
					ghttp.RespondWith(200, "{}")))
		})

		It("should stop the container", func() {
			err := connection.Stop("foo", true)
			Ω(err).ShouldNot(HaveOccurred())
		})
	})

	Describe("Limiting Memory", func() {
		Describe("setting the memory limit", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo/limits/memory"),
						verifyRequestBody(&garden.MemoryLimits{
							LimitInBytes: 42,
						}, &garden.MemoryLimits{}),
						ghttp.RespondWith(200, marshalProto(&garden.MemoryLimits{
							LimitInBytes: 40,
						})),
					),
				)
			})

			It("should limit memory", func() {
				newLimits, err := connection.LimitMemory("foo", garden.MemoryLimits{
					LimitInBytes: 42,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(newLimits.LimitInBytes).Should(BeNumerically("==", 40))
			})
		})

		Describe("getting the memory limit", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/memory"),
						ghttp.RespondWith(200, marshalProto(&garden.MemoryLimits{
							LimitInBytes: 40,
						}, &garden.MemoryLimits{})),
					),
				)
			})

			It("gets the memory limit", func() {
				currentLimits, err := connection.CurrentMemoryLimits("foo")
				Ω(err).ShouldNot(HaveOccurred())
				Ω(currentLimits.LimitInBytes).Should(BeNumerically("==", 40))
			})
		})
	})

	Describe("Limiting CPU", func() {
		Describe("setting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo/limits/cpu"),
						verifyRequestBody(&garden.CPULimits{
							LimitInShares: 42,
						}, &garden.CPULimits{}),
						ghttp.RespondWith(200, marshalProto(&garden.CPULimits{
							LimitInShares: 40,
						})),
					),
				)
			})

			It("should limit CPU", func() {
				newLimits, err := connection.LimitCPU("foo", garden.CPULimits{
					LimitInShares: 42,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(newLimits.LimitInShares).Should(BeNumerically("==", 40))
			})
		})

		Describe("getting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/cpu"),
						ghttp.RespondWith(200, marshalProto(&garden.CPULimits{
							LimitInShares: 40,
						})),
					),
				)
			})

			It("sends a nil cpu limit request", func() {
				limits, err := connection.CurrentCPULimits("foo")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(limits.LimitInShares).Should(BeNumerically("==", 40))
			})
		})
	})

	Describe("Limiting Bandwidth", func() {
		Describe("setting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo/limits/bandwidth"),
						verifyRequestBody(&garden.BandwidthLimits{
							RateInBytesPerSecond:      42,
							BurstRateInBytesPerSecond: 43,
						}, &garden.BandwidthLimits{}),
						ghttp.RespondWith(200, marshalProto(&garden.BandwidthLimits{
							RateInBytesPerSecond:      1,
							BurstRateInBytesPerSecond: 2,
						})),
					),
				)
			})

			It("should limit Bandwidth", func() {
				newLimits, err := connection.LimitBandwidth("foo", garden.BandwidthLimits{
					RateInBytesPerSecond:      42,
					BurstRateInBytesPerSecond: 43,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(newLimits.RateInBytesPerSecond).Should(BeNumerically("==", 1))
				Ω(newLimits.BurstRateInBytesPerSecond).Should(BeNumerically("==", 2))
			})
		})

		Describe("getting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/bandwidth"),
						ghttp.RespondWith(200, marshalProto(&garden.BandwidthLimits{
							RateInBytesPerSecond:      1,
							BurstRateInBytesPerSecond: 2,
						})),
					),
				)
			})

			It("sends a nil bandwidth limit request", func() {
				limits, err := connection.CurrentBandwidthLimits("foo")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(limits.RateInBytesPerSecond).Should(BeNumerically("==", 1))
				Ω(limits.BurstRateInBytesPerSecond).Should(BeNumerically("==", 2))
			})
		})
	})

	Describe("Limiting Disk", func() {
		Describe("setting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo/limits/disk"),
						verifyRequestBody(&garden.DiskLimits{
							BlockSoft: 42,
							BlockHard: 42,
							InodeSoft: 42,
							InodeHard: 42,
							ByteSoft:  42,
							ByteHard:  42,
						}, &garden.DiskLimits{}),
						ghttp.RespondWith(200, marshalProto(&garden.DiskLimits{
							BlockSoft: 3,
							BlockHard: 4,
							InodeSoft: 7,
							InodeHard: 8,
							ByteSoft:  11,
							ByteHard:  12,
						})),
					),
				)
			})

			It("should limit disk", func() {
				newLimits, err := connection.LimitDisk("foo", garden.DiskLimits{
					BlockSoft: 42,
					BlockHard: 42,

					InodeSoft: 42,
					InodeHard: 42,

					ByteSoft: 42,
					ByteHard: 42,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(newLimits).Should(Equal(garden.DiskLimits{
					BlockSoft: 3,
					BlockHard: 4,
					InodeSoft: 7,
					InodeHard: 8,
					ByteSoft:  11,
					ByteHard:  12,
				}))
			})
		})

		Describe("getting", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo/limits/disk"),
						ghttp.RespondWith(200, marshalProto(&garden.DiskLimits{
							BlockSoft: 3,
							BlockHard: 4,
							InodeSoft: 7,
							InodeHard: 8,
							ByteSoft:  11,
							ByteHard:  12,
						})),
					),
				)
			})

			It("sends a nil disk limit request", func() {
				limits, err := connection.CurrentDiskLimits("foo")
				Ω(err).ShouldNot(HaveOccurred())

				Ω(limits).Should(Equal(garden.DiskLimits{
					BlockSoft: 3,
					BlockHard: 4,
					InodeSoft: 7,
					InodeHard: 8,
					ByteSoft:  11,
					ByteHard:  12,
				}))
			})
		})
	})

	Describe("NetIn", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", "/containers/foo-handle/net/in"),
					verifyRequestBody(map[string]interface{}{
						"handle":         "foo-handle",
						"host_port":      float64(8080),
						"container_port": float64(8081),
					}, make(map[string]interface{})),
					ghttp.RespondWith(200, marshalProto(map[string]interface{}{
						"host_port":      1234,
						"container_port": 1235,
					}))))
		})

		It("should return the allocated ports", func() {
			hostPort, containerPort, err := connection.NetIn("foo-handle", 8080, 8081)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(hostPort).Should(Equal(uint32(1234)))
			Ω(containerPort).Should(Equal(uint32(1235)))
		})
	})

	Describe("NetOut", func() {
		var (
			rule   garden.NetOutRule
			handle string
		)

		BeforeEach(func() {
			handle = "foo-handle"
		})

		JustBeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("POST", fmt.Sprintf("/containers/%s/net/out", handle)),
					verifyRequestBody(&rule, &garden.NetOutRule{}),
					ghttp.RespondWith(200, "{}")))
		})

		Context("when a NetOutRule is passed", func() {
			BeforeEach(func() {
				rule = garden.NetOutRule{
					Protocol: garden.ProtocolICMP,
					Networks: []garden.IPRange{garden.IPRangeFromIP(net.ParseIP("1.2.3.4"))},
					Ports:    []garden.PortRange{garden.PortRangeFromPort(2), garden.PortRangeFromPort(4)},
					ICMPs:    &garden.ICMPControl{Type: 3, Code: garden.ICMPControlCode(3)},
					Log:      true,
				}
			})

			It("should send the rule over the wire", func() {
				Ω(connection.NetOut(handle, rule)).Should(Succeed())
			})
		})
	})

	Describe("Listing containers", func() {
		BeforeEach(func() {
			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/containers", "foo=bar"),
					ghttp.RespondWith(200, marshalProto(&struct {
						Handles []string `json:"handles"`
					}{
						[]string{"container1", "container2", "container3"},
					}))))
		})

		It("should return the list of containers", func() {
			handles, err := connection.List(map[string]string{"foo": "bar"})

			Ω(err).ShouldNot(HaveOccurred())
			Ω(handles).Should(Equal([]string{"container1", "container2", "container3"}))
		})
	})

	Describe("Getting container info", func() {
		var infoResponse garden.ContainerInfo

		JustBeforeEach(func() {
			infoResponse = garden.ContainerInfo{
				State: "chilling out",
				Events: []string{
					"maxing",
					"relaxing all cool",
				},
				HostIP:        "host-ip",
				ContainerIP:   "container-ip",
				ContainerPath: "container-path",
				ProcessIDs:    []uint32{1, 2},
				Properties: garden.Properties{
					"prop-key": "prop-value",
				},
				MemoryStat: garden.ContainerMemoryStat{
					Cache:                   1,
					Rss:                     2,
					MappedFile:              3,
					Pgpgin:                  4,
					Pgpgout:                 5,
					Swap:                    6,
					Pgfault:                 7,
					Pgmajfault:              8,
					InactiveAnon:            9,
					ActiveAnon:              10,
					InactiveFile:            11,
					ActiveFile:              12,
					Unevictable:             13,
					HierarchicalMemoryLimit: 14,
					HierarchicalMemswLimit:  15,
					TotalCache:              16,
					TotalRss:                17,
					TotalMappedFile:         18,
					TotalPgpgin:             19,
					TotalPgpgout:            20,
					TotalSwap:               21,
					TotalPgfault:            22,
					TotalPgmajfault:         23,
					TotalInactiveAnon:       24,
					TotalActiveAnon:         25,
					TotalInactiveFile:       26,
					TotalActiveFile:         27,
					TotalUnevictable:        28,
				},
				CPUStat: garden.ContainerCPUStat{
					Usage:  1,
					User:   2,
					System: 3,
				},

				DiskStat: garden.ContainerDiskStat{
					BytesUsed:  1,
					InodesUsed: 2,
				},

				MappedPorts: []garden.PortMapping{
					{HostPort: 1234, ContainerPort: 5678},
					{HostPort: 1235, ContainerPort: 5679},
				},
			}

			server.AppendHandlers(
				ghttp.CombineHandlers(
					ghttp.VerifyRequest("GET", "/containers/some-handle/info"),
					ghttp.RespondWith(200, marshalProto(infoResponse))))
		})

		It("should return the container's info", func() {
			info, err := connection.Info("some-handle")
			Ω(err).ShouldNot(HaveOccurred())

			Ω(info).Should(Equal(infoResponse))
		})
	})

	Describe("Streaming in", func() {
		Context("when streaming in succeeds", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo-handle/files", "destination=%2Fbar"),
						func(w http.ResponseWriter, r *http.Request) {
							body, err := ioutil.ReadAll(r.Body)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(string(body)).Should(Equal("chunk-1chunk-2"))
						},
					),
				)
			})

			It("tells garden.to stream, and then streams the content as a series of chunks", func() {
				buffer := bytes.NewBufferString("chunk-1chunk-2")

				err := connection.StreamIn("foo-handle", "/bar", buffer)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when streaming in returns an error response", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo-handle/files", "destination=%2Fbar"),
						ghttp.RespondWith(http.StatusInternalServerError, "no."),
					),
				)
			})

			It("returns an error on close", func() {
				buffer := bytes.NewBufferString("chunk-1chunk-2")
				err := connection.StreamIn("foo-handle", "/bar", buffer)
				Ω(err).Should(HaveOccurred())

				Ω(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})

		Context("when streaming in fails hard", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("PUT", "/containers/foo-handle/files", "destination=%2Fbar"),
						ghttp.RespondWith(http.StatusInternalServerError, "no."),
						func(w http.ResponseWriter, r *http.Request) {
							server.CloseClientConnections()
						},
					),
				)
			})

			It("returns an error on close", func() {
				buffer := bytes.NewBufferString("chunk-1chunk-2")

				err := connection.StreamIn("foo-handle", "/bar", buffer)
				Ω(err).Should(HaveOccurred())

				Ω(server.ReceivedRequests()).Should(HaveLen(1))
			})
		})
	})

	Describe("Streaming Out", func() {
		Context("when streaming succeeds", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/files", "source=%2Fbar"),
						ghttp.RespondWith(200, "hello-world!"),
					),
				)
			})

			It("asks garden.for the given file, then reads its content", func() {
				reader, err := connection.StreamOut("foo-handle", "/bar")
				Ω(err).ShouldNot(HaveOccurred())

				readBytes, err := ioutil.ReadAll(reader)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(readBytes).Should(Equal([]byte("hello-world!")))

				reader.Close()
			})
		})

		Context("when streaming fails", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/files", "source=%2Fbar"),
						func(w http.ResponseWriter, r *http.Request) {
							w.Header().Set("Content-Length", "500")
						},
					),
				)
			})

			It("asks garden.for the given file, then reads its content", func() {
				reader, err := connection.StreamOut("foo-handle", "/bar")
				Ω(err).ShouldNot(HaveOccurred())

				_, err = ioutil.ReadAll(reader)
				reader.Close()
				Ω(err).Should(HaveOccurred())
			})
		})
	})

	Describe("Running", func() {
		var spec garden.ProcessSpec

		Context("when streaming succeeds to completion", func() {
			BeforeEach(func() {
				spec = garden.ProcessSpec{
					Path:       "lol",
					Args:       []string{"arg1", "arg2"},
					Dir:        "/some/dir",
					Privileged: true,
					Limits:     resourceLimits,
				}

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						ghttp.VerifyJSONRepresenting(spec),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, map[string]interface{}{"process_id": 42})
							transport.WriteMessage(conn, map[string]interface{}{"process_id": 42, "source": transport.Stdout, "data": "stdout data"})
							transport.WriteMessage(conn, map[string]interface{}{"process_id": 42, "source": transport.Stderr, "data": "stderr data"})

							var payload map[string]interface{}
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(map[string]interface{}{
								"process_id": float64(42),
								"source":     float64(transport.Stdin),
								"data":       "stdin data",
							}))

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stdout,
								"data":       fmt.Sprintf("roundtripped %s", payload["data"]),
							})

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  42,
								"exit_status": 3,
							})
						},
					),
				)
			})

			It("streams the data, closes the destinations, and notifies of exit", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()

				process, err := connection.Run("foo-handle", spec, garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: stdout,
					Stderr: stderr,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				Eventually(stdout).Should(gbytes.Say("stdout data"))
				Eventually(stdout).Should(gbytes.Say("roundtripped stdin data"))
				Eventually(stderr).Should(gbytes.Say("stderr data"))

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when the process is terminated", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
							})

							var payload map[string]interface{}
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(map[string]interface{}{
								"process_id": float64(42),
								"signal":     float64(garden.SignalTerminate),
							}))

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  42,
								"exit_status": 3,
							})
						},
					),
				)
			})

			It("sends the appropriate protocol message", func() {
				process, err := connection.Run("foo-handle", garden.ProcessSpec{}, garden.ProcessIO{})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				err = process.Signal(garden.SignalTerminate)
				Ω(err).ShouldNot(HaveOccurred())

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when the process is killed", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
							})

							var payload map[string]interface{}
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(map[string]interface{}{
								"process_id": float64(42),
								"signal":     float64(garden.SignalKill),
							}))

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  42,
								"exit_status": 3,
							})
						},
					),
				)
			})

			It("sends the appropriate protocol message", func() {
				process, err := connection.Run("foo-handle", garden.ProcessSpec{}, garden.ProcessIO{})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				err = process.Signal(garden.SignalKill)
				Ω(err).ShouldNot(HaveOccurred())

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when the process's window is resized", func() {
			var spec garden.ProcessSpec
			BeforeEach(func() {
				spec = garden.ProcessSpec{
					Path: "lol",
					Args: []string{"arg1", "arg2"},
					TTY: &garden.TTYSpec{
						WindowSize: &garden.WindowSize{
							Columns: 100,
							Rows:    200,
						},
					},
				}

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						ghttp.VerifyJSONRepresenting(spec),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							decoder := json.NewDecoder(br)

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
							})

							var payload map[string]interface{}
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(map[string]interface{}{
								"process_id": float64(42),
								"tty": map[string]interface{}{
									"window_size": map[string]interface{}{
										"columns": float64(80),
										"rows":    float64(24),
									},
								},
							}))

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  42,
								"exit_status": 3,
							})
						},
					),
				)
			})

			It("sends the appropriate protocol message", func() {
				process, err := connection.Run("foo-handle", spec, garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: gbytes.NewBuffer(),
					Stderr: gbytes.NewBuffer(),
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				err = process.SetTTY(garden.TTYSpec{
					WindowSize: &garden.WindowSize{
						Columns: 80,
						Rows:    24,
					},
				})
				Ω(err).ShouldNot(HaveOccurred())

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when the connection breaks before an exit status is received", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, _, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
							})
							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stdout,
								"data":       "stdout data",
							})
							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stderr,
								"data":       "stderr data",
							})
						},
					),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Run("foo-handle", garden.ProcessSpec{
						Path: "lol",
						Args: []string{"arg1", "arg2"},
						Dir:  "/some/dir",
					}, garden.ProcessIO{})

					Ω(err).ShouldNot(HaveOccurred())

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
				})
			})
		})

		Context("when the connection returns an error payload", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("POST", "/containers/foo-handle/processes"),
						ghttp.RespondWith(200, marshalProto(map[string]interface{}{
							"process_id": 42,
						},
							map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stdout,
								"data":       "stdout data",
							},
							map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stderr,
								"data":       "stderr data",
							},
							map[string]interface{}{
								"process_id": 42,
								"error":      "oh no!",
							},
						)),
					),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Run("foo-handle", garden.ProcessSpec{
						Path: "lol",
						Args: []string{"arg1", "arg2"},
						Dir:  "/some/dir",
					}, garden.ProcessIO{})

					Ω(err).ShouldNot(HaveOccurred())

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
					Ω(err.Error()).Should(ContainSubstring("oh no!"))
				})
			})
		})
	})

	Describe("Attaching", func() {
		Context("when streaming succeeds to completion", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/42"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stdout,
								"data":       "stdout data",
							})

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stderr,
								"data":       "stderr data",
							})

							var payload map[string]interface{}
							err = json.NewDecoder(br).Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(map[string]interface{}{
								"process_id": float64(42),
								"source":     float64(transport.Stdin),
								"data":       "stdin data",
							}))

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stdout,
								"data":       fmt.Sprintf("roundtripped %s", payload["data"]),
							})

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id":  42,
								"exit_status": 3,
							})
						},
					),
				)
			})

			It("should stream", func() {
				stdout := gbytes.NewBuffer()
				stderr := gbytes.NewBuffer()

				process, err := connection.Attach("foo-handle", 42, garden.ProcessIO{
					Stdin:  bytes.NewBufferString("stdin data"),
					Stdout: stdout,
					Stderr: stderr,
				})

				Ω(err).ShouldNot(HaveOccurred())
				Ω(process.ID()).Should(Equal(uint32(42)))

				Eventually(stdout).Should(gbytes.Say("stdout data"))
				Eventually(stderr).Should(gbytes.Say("stderr data"))
				Eventually(stdout).Should(gbytes.Say("roundtripped stdin data"))

				status, err := process.Wait()
				Ω(err).ShouldNot(HaveOccurred())
				Ω(status).Should(Equal(3))
			})
		})

		Context("when an error occurs while reading the given stdin stream", func() {
			It("does not send an EOF to close the process's stdin", func() {
				finishedReq := make(chan struct{})

				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/42"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, br, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()
							decoder := json.NewDecoder(br)

							var payload map[string]interface{}
							err = decoder.Decode(&payload)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(payload).Should(Equal(map[string]interface{}{
								"process_id": float64(42),
								"source":     float64(transport.Stdin),
								"data":       "stdin data",
							}))

							var payload2 map[string]interface{}
							err = decoder.Decode(&payload2)
							Ω(err).Should(HaveOccurred())

							close(finishedReq)
						},
					),
				)

				stdinR, stdinW := io.Pipe()

				_, err := connection.Attach("foo-handle", 42, garden.ProcessIO{
					Stdin: stdinR,
				})
				Ω(err).ShouldNot(HaveOccurred())

				stdinW.Write([]byte("stdin data"))
				stdinW.CloseWithError(errors.New("connection broke"))

				Eventually(finishedReq).Should(BeClosed())
			})
		})

		Context("when the connection returns an error payload", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/42"),
						ghttp.RespondWith(200, marshalProto(map[string]interface{}{
							"process_id": 42,
						},
							map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stdout,
								"data":       "stdout data",
							},
							map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stderr,
								"data":       "stderr data",
							},
							map[string]interface{}{
								"process_id": 42,
								"error":      "oh no!",
							},
						)),
					),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Attach("foo-handle", 42, garden.ProcessIO{})

					Ω(err).ShouldNot(HaveOccurred())
					Ω(process.ID()).Should(Equal(uint32(42)))

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
					Ω(err.Error()).Should(ContainSubstring("oh no!"))
				})
			})
		})

		Context("when the connection breaks before an exit status is received", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					ghttp.CombineHandlers(
						ghttp.VerifyRequest("GET", "/containers/foo-handle/processes/42"),
						func(w http.ResponseWriter, r *http.Request) {
							w.WriteHeader(http.StatusOK)

							conn, _, err := w.(http.Hijacker).Hijack()
							Ω(err).ShouldNot(HaveOccurred())

							defer conn.Close()

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stdout,
								"data":       "stdout data",
							})

							transport.WriteMessage(conn, map[string]interface{}{
								"process_id": 42,
								"source":     transport.Stderr,
								"data":       "stderr data",
							})
						},
					),
				)
			})

			Describe("waiting on the process", func() {
				It("returns an error", func() {
					process, err := connection.Attach("foo-handle", 42, garden.ProcessIO{})

					Ω(err).ShouldNot(HaveOccurred())
					Ω(process.ID()).Should(Equal(uint32(42)))

					_, err = process.Wait()
					Ω(err).Should(HaveOccurred())
				})
			})
		})
	})
})

func verifyRequestBody(expectedMessage interface{}, emptyType interface{}) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		defer GinkgoRecover()

		decoder := json.NewDecoder(req.Body)

		received := emptyType
		err := decoder.Decode(&received)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(received).Should(Equal(expectedMessage))
	}
}

func marshalProto(messages ...interface{}) string {
	result := new(bytes.Buffer)
	for _, msg := range messages {
		err := transport.WriteMessage(result, msg)
		Ω(err).ShouldNot(HaveOccurred())
	}

	return result.String()
}
