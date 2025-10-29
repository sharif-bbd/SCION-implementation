package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"time"

	"github.com/scionproto/scion/pkg/addr"
	"github.com/scionproto/scion/pkg/daemon"

	"github.com/scionproto/scion/pkg/log"

	"github.com/scionproto/scion/pkg/private/serrors"
	"github.com/scionproto/scion/pkg/snet"
	"github.com/scionproto/scion/pkg/snet/path"

	"github.com/scionproto/scion/private/path/fabridquery"
	"gitlab.inf.ethz.ch/PRV-PERRIG/netsec-course/project-scion/lib"
)

// The local IP address of your endhost.
// It matches the IP address of the SCION daemon you should use for this run.
var local string

// The remote SCION address of the verifier application.
var remote snet.UDPAddr

// The port of your SCION daemon.
const daemonPort = 30255

func main() {
	// DO NOT MODIFY THIS FUNCTION
	err := log.Setup(log.Config{
		Console: log.ConsoleConfig{
			Level:           "DEBUG",
			StacktraceLevel: "none",
		},
	})
	if err != nil {
		fmt.Println(serrors.WrapStr("setting up logging", err))
	}
	flag.StringVar(&local, "local", "", "The local IP address which is the same IP as the IP of the local SCION daemon")
	flag.Var(&remote, "remote", "The address of the validator")
	flag.Parse()

	if err := realMain(); err != nil {
		log.Error("Error while running project", "err", err)
	}
}

func connectDaemon(local string) (daemon.Connector, error){
	ip, err := netip.ParseAddr(local)
    if err != nil {
        return nil, fmt.Errorf("parse local ip: %w", err)
    }
    var daemonAddr string
    if ip.Is6() {
        daemonAddr = fmt.Sprintf("[%s]:%d", local, daemon.DefaultAPIPort)
    } else {
        daemonAddr = fmt.Sprintf("%s:%d", local, daemon.DefaultAPIPort)
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    return daemon.NewService(daemonAddr).Connect(ctx)
}

func findPath(sd daemon.Connector, ctx context.Context, localIA addr.IA, test_id lib.TestID) ([]snet.Path, error){
	fmt.Printf("Finding paths for test : %v\n", test_id)
	var paths []snet.Path
	var err error
	if test_id == lib.EpicHiddenPathTest{
		paths, err = sd.Paths(ctx, remote.IA, localIA, daemon.PathReqFlags{Hidden: true})
		if len(paths) == 0 || err != nil{
			paths, err = sd.Paths(ctx, remote.IA, localIA, daemon.PathReqFlags{})
		}
	}else{
		
		paths, err = sd.Paths(ctx, remote.IA, localIA, daemon.PathReqFlags{})
	}
	if err != nil || len(paths) == 0{
		return nil, fmt.Errorf("failed to get paths: %w",err)
	}


	for i:=0 ; i < len(paths);i++{
		fmt.Printf("Path %d: ", i)
		fmt.Println(paths[i])
		fmt.Println("Supports EPIC", paths[i].Metadata().EpicAuths.SupportsEpic())
		fmt.Printf("Interfaces : ")
		interfaces := paths[i].Metadata().Interfaces
		for j := 0; j < len(interfaces); j++{
			fmt.Printf("%d, ", interfaces[j].ID)
		}
		fmt.Println("")
		fmt.Println("Latencies : ", paths[i].Metadata().Latency)
		fmt.Println("Bandwidths : ", paths[i].Metadata().Bandwidth)
	}

	return paths, nil
}

func test01(sd daemon.Connector, ctx context.Context, localIA addr.IA, scionNet snet.SCIONNetwork, listen *net.UDPAddr) error{
	paths, err := findPath(sd, ctx, localIA, lib.BasicConnectivityTest)
	if err != nil {
		return fmt.Errorf("could not find paths to the remote : %w", err)
	}
	p := paths[0]

	remote.Path = p.Dataplane()
	remote.NextHop = p.UnderlayNextHop()
	
	
	conn, err :=scionNet.Dial(ctx, "udp", listen, &remote)
	if err != nil{
		return fmt.Errorf("could not establish a connection: %w", err)
	}
	defer conn.Close()

	msg := lib.Test{ID: lib.BasicConnectivityTest, Payload: ""}
	m, _ := json.Marshal(msg)

	fmt.Println(m)

	_, err = conn.Write(m)
	if err != nil{
		return fmt.Errorf("couldn't write the message: %w", err)
	}

	buf := make([]byte, 2048)
	n,err := conn.Read(buf)
	if err != nil{
		return fmt.Errorf("could not read answer: %w", err)
	}
	if n > 2048{
		return fmt.Errorf("message longer than buffer")
	}

	var res lib.TestResult
	if err := json.Unmarshal(buf[:n], &res); err != nil{
		fmt.Println("Raw reply:", string(buf[:n]))
		return nil
	}
	fmt.Printf("Verifier Replied : ID=%d State=%s \n", res.ID, res.State)
	return nil
}


func trickedType(payload any) (int, error){
	var need int
	switch v:=payload.(type){
	case float64:
		if v < 0 || v > float64(int(^uint(0)>>1)){
			return -1, fmt.Errorf("payload out of range of int: %v", v)
		}
		need = int(v)
	case int:
		need = v
	case uint:
		need = int(v)
	default:
		return -1, fmt.Errorf("unexpected payload type %T (%v)", v, v)
	}
	return need, nil
}

func test02(sd daemon.Connector, ctx context.Context, localIA addr.IA, scionNet snet.SCIONNetwork, listen *net.UDPAddr) error{
	paths, err := findPath(sd, ctx, localIA, lib.BasicMultipathTest)
	if err != nil {
		return fmt.Errorf("could not query the paths : %w", err)
	}

	p := paths[0]

	remote.Path = p.Dataplane()
	remote.NextHop = p.UnderlayNextHop()

	conn, err := scionNet.Dial(ctx, "udp", listen, &remote)
	if err != nil{
		return fmt.Errorf("couldn't establish a connection : %w", err)
	}
	defer conn.Close()

	msg := lib.Test{ID: lib.BasicMultipathTest, Payload: ""}
	m, _:=json.Marshal(msg)
	fmt.Println(m)

	_, err =conn.Write(m)
	if err != nil {
		return fmt.Errorf("couldn't write message : %w", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil{
		return fmt.Errorf("could not read answer : %w", err)
	}
	if n > 2048{
		return fmt.Errorf("message longer than buffer")
	}
	var res lib.TestResult
	if err := json.Unmarshal(buf[:n], &res); err != nil{
		fmt.Println("Raw reply:", string(buf[:n]))
		return nil
	}
	fmt.Printf("Verifier Replied: ID= %d, Payload= %f, State= %s\n", res.ID, res.Payload, res.State)
	if res.State == lib.TestPassed{
		return nil
	}

	// Trick the program into assigning uint to the Payload

	multi, err := trickedType(res.Payload)
	if err != nil{
		return fmt.Errorf("payload isn't a number: %w", err)
	}

	if multi+1 > len(paths){
		return fmt.Errorf("too many paths demanded by the verifier")
	}

	for i:=0; i < multi; i++{
		fmt.Printf("----------------- PATH %d -----------------", i+1)
		p := paths[i+1]
		remote.Path = p.Dataplane()
		remote.NextHop = p.UnderlayNextHop()

		conn, err := scionNet.Dial(ctx, "udp", listen, &remote)
		if err != nil{
			return fmt.Errorf("couldn't establish a connection : %w", err)
		}
		defer conn.Close()

		msg := lib.Test{ID: lib.BasicMultipathTest, Payload: ""}
		m, _:=json.Marshal(msg)
		fmt.Println(m)

		_, err =conn.Write(m)
		if err != nil {
			return fmt.Errorf("couldn't write message : %w", err)
		}

		buf := make([]byte, 2048)
		n, err := conn.Read(buf)
		if err != nil{
			return fmt.Errorf("could not read answer : %w", err)
		}
		if n > 2048{
			return fmt.Errorf("message longer than buffer")
		}
		var res lib.TestResult
		if err := json.Unmarshal(buf[:n], &res); err != nil{
			fmt.Println("Raw reply:", string(buf[:n]))
			return nil
		}
		fmt.Printf("Verifier Replied: ID= %d, Payload= %f, State= %s\n", res.ID, res.Payload, res.State)
		if res.State == lib.TestPassed{
			return nil
		}	
	}


	return nil
}



func lowestCarbonIndex(carbon [][]int64) int{

	if len(carbon) == 0 {
		return -1
	}
	fmt.Println(carbon)

	minMissing := int(^uint(0) >> 1)
	missing := make([]int, len(carbon))
	for i, row := range carbon {
		c := 0
		for _, v := range row {
			if v < 0 {
				c++
			}
		}
		missing[i] = c
		if c < minMissing {
			minMissing = c
		}
	}

	best := -1
	var bestSum int64
	first := true
	for i, row := range carbon {
		if missing[i] != minMissing {
			continue
		}
		var s int64
		for _, v := range row {
			if v >= 0 {
				s += v
			}
		}
		if first || s < bestSum {
			best = i
			bestSum = s
			first = false
		}
	}
	fmt.Println(best)
	return best
}



func test10(sd daemon.Connector, ctx context.Context, localIA addr.IA, scionNet snet.SCIONNetwork, listen *net.UDPAddr) error{
	paths, err := findPath(sd, ctx, localIA, lib.MinimizeCarbonIntensity)
	if err != nil {
		return fmt.Errorf("couldn't fetch any paths : %w", err)
	}

	var carbon [][]int64
	for i := 0; i < len(paths); i++{
		carbon = append(carbon, paths[i].Metadata().CarbonIntensity)
	}
	
	pathIdx := lowestCarbonIndex(carbon)
	if pathIdx == -1{
		return fmt.Errorf("couldn't find the path with lowest carbon intensity")
	}
	
	p := paths[pathIdx]

	remote.Path = p.Dataplane()
	remote.NextHop = p.UnderlayNextHop()
	
	
	conn, err :=scionNet.Dial(ctx, "udp", listen, &remote)
	if err != nil{
		return fmt.Errorf("could not establish a connection: %w", err)
	}
	defer conn.Close()

	msg := lib.Test{ID: lib.MinimizeCarbonIntensity, Payload: ""}
	m, _ := json.Marshal(msg)

	fmt.Println(m)

	_, err = conn.Write(m)
	if err != nil{
		return fmt.Errorf("couldn't write the message: %w", err)
	}

	buf := make([]byte, 2048)
	n,err := conn.Read(buf)
	if err != nil{
		return fmt.Errorf("could not read answer: %w", err)
	}
	if n > 2048{
		return fmt.Errorf("message longer than buffer")
	}

	var res lib.TestResult
	if err := json.Unmarshal(buf[:n], &res); err != nil{
		fmt.Println("Raw reply:", string(buf[:n]))
		return nil
	}
	fmt.Printf("Verifier Replied : ID=%d State=%s \n", res.ID, res.State)
	return nil

}


func findShortestPaths(paths []snet.Path) []snet.Path{
	if len(paths) == 0{
		return nil
	}
	if len(paths) == 1{
		return paths
	}

	lengths := make(map[int][]snet.Path)
	minLength := int(^uint(0)>>1)

	for i:= 0; i < len(paths); i++{
		inter := paths[i].Metadata().Interfaces

		n :=len(inter)
		lengths[n] = append(lengths[n], paths[i])
		if n < minLength{
			minLength = n
		}
	}

	shortest := lengths[minLength]
	if len(shortest)<=1{
		return shortest
	}

	sort.Slice(shortest, func(i,j int) bool {
		return comparePathsByInterfaceIDs(shortest[i], shortest[j]) < 0
	})

	return shortest

}

func comparePathsByInterfaceIDs(a,b snet.Path) int{
	ma, mb := a.Metadata(), b.Metadata()

	if ma == nil && mb != nil{
		return 1

	}
	if ma != nil && mb == nil{
		return -1
	}
	if ma == nil &&mb ==nil{
		return 0
	}

	aInter, bInter := ma.Interfaces, mb.Interfaces

	min := len(aInter)
	if len(bInter) < min{
		min  = len(bInter)
	}
	for i := 0; i < min; i++{

		if aInter[i].ID < bInter[i].ID{
			return -1
		}
		if aInter[i].ID > bInter[i].ID{
			return 1
		}
	}

	if len(aInter) < len(bInter){
		return -1
	}
	if len(aInter) > len(bInter){
		return 1
	}
	return 0
}

func fetchEPICPaths(paths []snet.Path) ([]snet.Path, error){
	var epicPaths []snet.Path

	for i:= 0; i < len(paths); i++{
		if paths[i].Metadata().EpicAuths.SupportsEpic(){
			epicPaths = append(epicPaths, paths[i])
		}
	}

	if len(epicPaths) != 0{
		return epicPaths, nil
	}
	
	return nil, nil
}


func test20(sd daemon.Connector, ctx context.Context, localIA addr.IA, scionNet snet.SCIONNetwork, listen *net.UDPAddr) error{
	paths, err := findPath(sd, ctx, localIA, lib.EpicHiddenPathTest)
	if err != nil{
		return fmt.Errorf("couldn't fetch any paths : %w", err)
	}

	
	if epicPaths, _ := fetchEPICPaths(paths); len(epicPaths) > 0 {
		fmt.Println("Length of epic paths: ", len(epicPaths))
		paths = epicPaths
	}

	paths = findShortestPaths(paths)
	if len(paths) == 0{
		return fmt.Errorf("no valid paths found")
	}
	fmt.Println(paths)


	p := paths[0]
	
	if p.Metadata().EpicAuths.SupportsEpic(){
		if scionDP, ok := p.Dataplane().(path.SCION); ok{
			if epicDP, err := path.NewEPICDataplanePath(scionDP, p.Metadata().EpicAuths); err == nil{
				remote.Path = epicDP
			}else{
				remote.Path = p.Dataplane()
			}
		}else{
			remote.Path = p.Dataplane()
		}
	}else{
		remote.Path = p.Dataplane()
	}

	remote.NextHop = p.UnderlayNextHop()
	
	
	conn, err :=scionNet.Dial(ctx, "udp", listen, &remote)
	if err != nil{
		return fmt.Errorf("could not establish a connection: %w", err)
	}
	defer conn.Close()

	msg := lib.Test{ID: lib.EpicHiddenPathTest, Payload: ""}
	m, _ := json.Marshal(msg)

	fmt.Println(m)

	_, err = conn.Write(m)
	if err != nil{
		return fmt.Errorf("couldn't write the message: %w", err)
	}

	buf := make([]byte, 2048)
	n,err := conn.Read(buf)
	if err != nil{
		return fmt.Errorf("could not read answer: %w", err)
	}
	if n > 2048{
		return fmt.Errorf("message longer than buffer")
	}

	var res lib.TestResult
	if err := json.Unmarshal(buf[:n], &res); err != nil{
		fmt.Println("Raw reply:", string(buf[:n]))
		return err
	}
	fmt.Printf("Verifier Replied : ID=%d State=%s \n", res.ID, res.State)
	return nil

}


func sumLatency(lat []time.Duration) time.Duration{
	var res time.Duration
	for i := 0 ; i < len(lat); i++{
		if lat[i] >=0{
			res += lat[i]
		}
	}

	return res
}

func minBandwidth(bw []uint64) uint64{
	min := uint64(^uint(0)>>1)
	for i := 0; i < len(bw) ; i++{
		if bw[i] < min && bw[i] != 0{
			min = bw[i]
		}
	}

	return min
}

func getLatBound(sd daemon.Connector, ctx context.Context, localIA addr.IA, scionNet snet.SCIONNetwork, listen *net.UDPAddr, p snet.Path) (int, error){

	remote.Path = p.Dataplane()
	remote.NextHop = p.UnderlayNextHop()

	conn, err := scionNet.Dial(ctx, "udp", listen, &remote)
	if err != nil{
		return -1, fmt.Errorf("couldn't establish a connection : %w", err)
	}
	defer conn.Close()

	msg := lib.Test{ID: lib.MaximizeBandwidthWithBoundedLatency, Payload: ""}
	m, _:=json.Marshal(msg)
	fmt.Println(m)

	_, err =conn.Write(m)
	if err != nil {
		return -1, fmt.Errorf("couldn't write message : %w", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil{
		return -1, fmt.Errorf("could not read answer : %w", err)
	}
	if n > 2048{
		return -1, fmt.Errorf("message longer than buffer")
	}
	var res lib.TestResult
	if err := json.Unmarshal(buf[:n], &res); err != nil{
		fmt.Println("Raw reply:", string(buf[:n]))
		return -1, nil
	}
	fmt.Printf("Verifier Replied: ID= %d, Payload= %f, State= %s\n", res.ID, res.Payload, res.State)
	
	// Trick the program into assigning uint to the Payload

	bound, err := trickedType(res.Payload)
	if err != nil{
		return -1, fmt.Errorf("error casting payload into int")
	}

	fmt.Println("Latency Bound : ", time.Duration(bound*1000*1000))
	return bound*1000*1000, nil

}

func undefinedBandwidthCount(path snet.Path) int{
	var count int
	bw := path.Metadata().Bandwidth
	for i := 0 ; i < len(bw); i++{
		if bw[i] == 0{
			count++
		}
	}

	if count == len(bw){
		count = -1
	}
	return count
}

func undefinedLatCount(path snet.Path) int{
	var count int
	lat := path.Metadata().Latency
	for i := 0; i < len(lat); i++{
		if lat[i] < 0{
			count++
		}
	}
	return count
}

func validPaths(paths []snet.Path, latBound int) []snet.Path{

	var res []snet.Path
	minBwMissing, minLatMissing := int(^uint(0)>>1), int(^uint(0)>>1)
	missing := make(map[int][]int)
	var lats []time.Duration
	var bands []uint64
	allLats := true


	// Register the lowest missing latency among these paths
	for i, path := range paths{
		latSum := sumLatency(path.Metadata().Latency)
		lats = append(lats, latSum)
		bands = append(bands, minBandwidth(path.Metadata().Bandwidth))
		countLat := undefinedLatCount(path)

		misses := []int{countLat}
		missing[i] = misses

		if countLat < minLatMissing && latSum.Milliseconds() <= time.Duration(latBound).Milliseconds(){
			minLatMissing = countLat
		}		
		
	}

	if minLatMissing > 0{
		allLats = false
	}

	fmt.Println("Missing map after adding latency counts : ", missing)


	//min amount of missing Bandwidths for the min amount of missing latencies
	for i, path := range paths{
		if missing[i][0] != minLatMissing || lats[i].Milliseconds() > time.Duration(latBound).Milliseconds() {
			continue
		}
		countBw := undefinedBandwidthCount(path)
		missing[i] = append(missing[i], countBw)
		
		if countBw < minBwMissing{
			minBwMissing = countBw
		}

	}

	fmt.Println("Missing map after adding bandwidths count : ", missing)

	var maxBw uint64
	maxBw = 0

	//Identify the max Bandwidth available 
	for i := range paths{
		if missing[i][0] !=  minLatMissing ||  lats[i].Milliseconds() > time.Duration(latBound).Milliseconds(){
			continue
		}
		if missing[i][1] != minBwMissing && allLats  {
			continue
		}
		if bands[i] > maxBw{
			maxBw = bands[i]
		}

	}

	fmt.Println("All latencies : ", lats)
	fmt.Println("All bandwidths : ", bands)
	fmt.Println("Max Bandwidth : ", maxBw)
	fmt.Println("Min Missing Latencies", minLatMissing)
	fmt.Println("Min Missing Bandwidths", minBwMissing)

	// Keep only the paths with the minimum lat and bw missing, that respect the lat bound and that maximizes the bandwidth
	for i, path := range paths{
		if missing[i][0] != minLatMissing ||  lats[i].Milliseconds() > time.Duration(latBound).Milliseconds() || bands[i] != maxBw{
			continue
		}
		if missing[i][1] != minBwMissing && allLats{
			continue
		}
		fmt.Printf("Adding path number %d\n", i)
		res = append(res, path)
	}

	return res

}


func test11(sd daemon.Connector, ctx context.Context, localIA addr.IA, scionNet snet.SCIONNetwork, listen *net.UDPAddr) error{
	paths, err := findPath(sd, ctx, localIA, lib.MaximizeBandwidthWithBoundedLatency)
	if err != nil{
		return fmt.Errorf("couldn't find paths to remote : %w", err)
	}

	latBound, err := getLatBound(sd, ctx, localIA, scionNet, listen, paths[0])
	if err != nil{
		return fmt.Errorf("couldn't get the latency bound from remote : %w", err)
	}

	paths = validPaths(paths, latBound)
	fmt.Println("Length after validPaths() : ", len(paths))
	paths = findShortestPaths(paths)
	fmt.Println("Length after findShortestPath() : ", len(paths))
	p := paths[0]

	
	remote.Path = p.Dataplane()
	remote.NextHop = p.UnderlayNextHop()

	conn, err := scionNet.Dial(ctx, "udp", listen, &remote)
	if err != nil{
		return fmt.Errorf("couldn't establish a connection : %w", err)
	}
	defer conn.Close()

	msg := lib.Test{ID: lib.MaximizeBandwidthWithBoundedLatency, Payload: ""}
	m, _:=json.Marshal(msg)
	fmt.Println(m)

	_, err =conn.Write(m)
	if err != nil {
		return fmt.Errorf("couldn't write message : %w", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil{
		return fmt.Errorf("could not read answer : %w", err)
	}
	if n > 2048{
		return fmt.Errorf("message longer than buffer")
	}
	var res lib.TestResult
	if err := json.Unmarshal(buf[:n], &res); err != nil{
		fmt.Println("Raw reply:", string(buf[:n]))
		return nil
	}
	fmt.Printf("Verifier Replied: ID= %d, Payload= %f, State= %s\n", res.ID, res.Payload, res.State)

	return nil
}

func checkFabridEnabled(paths []snet.Path, hops [][]snet.HopInterface) []snet.Path{
	var res []snet.Path
	for i, path := range paths{
		fabridEnabled := true
		for _, hop := range hops[i]{
			if !hop.FabridEnabled{
				fabridEnabled = false
			}
		}
		if fabridEnabled{
			res = append(res, path)
		}
	}
	return res
}


func test30(sd daemon.Connector, ctx context.Context, localIA addr.IA, scionNet snet.SCIONNetwork, listen *net.UDPAddr) error{
	paths, err := findPath(sd, ctx, localIA, lib.FabridConnectivityTest)
	if err != nil {
		return fmt.Errorf("couldn't fetch paths : %w", err)
	}

	var hops [][]snet.HopInterface

	for i, path := range paths{
		hops = append(hops, path.Metadata().Hops())
		fmt.Printf("Fabrid Info for Path %d\nEnabled: ", i)
		fabInfo := path.Metadata().FabridInfo
		for _, f := range fabInfo{
			fmt.Printf("%t, ", f.Enabled)
		}
		fmt.Println("")
		fmt.Printf("Policies : ")
		for _, f := range fabInfo{
			fmt.Println(f.Policies)
		}
		fmt.Println("Hops : ", hops[i])
		
	}
	
	paths = checkFabridEnabled(paths, hops)
	if len(paths) == 0{
		return fmt.Errorf("none of the Paths support Fabrid")
	}
	paths = findShortestPaths(paths)
	if len(paths) == 0{
		return fmt.Errorf("no FABRID-enabled paths found")
	}
	p := paths[0]
	selectedHops := p.Metadata().Hops()
	fmt.Println("Chosen Path : ", p)
	fmt.Println("Selected Hops : ", selectedHops)
	input := "0-0#0,0@0"
	exp,err := fabridquery.ParseFabridQuery(input)
	if err != nil{
		return fmt.Errorf("couldn't parse fabrid query string : %w", err)
	}
	var ml *fabridquery.MatchList
	ml = &fabridquery.MatchList{
		SelectedPolicies: make([]*fabridquery.Policy, len(selectedHops)),
	}

	_, ml = exp.Evaluate(selectedHops, ml)

	fmt.Println("MatchList : ", ml)
	policyIDs := ml.Policies()

	scionDP, ok := p.Dataplane().(path.SCION)
	if !ok{
		return fmt.Errorf("path is not a SCION dataplane path")
	}

	fabridConfig := &path.FabridConfig{
		LocalIA: localIA,
		LocalAddr: local,
		DestinationIA: remote.IA,
		DestinationAddr: remote.Host.IP.String(),
	}
	
	fabridDPP, err := path.NewFABRIDDataplanePath(
		scionDP,
		selectedHops,
		policyIDs,
		fabridConfig,
		sd.FabridKeys,
	)
	
	if err != nil{
		return fmt.Errorf("couldn't create FABRID dataplane path : %w", err)
	}

	remote.Path = fabridDPP
	remote.NextHop = p.UnderlayNextHop()

	conn, err := scionNet.Dial(ctx, "udp", listen, &remote)
	if err != nil{
		return fmt.Errorf("could not establish a connection : %w", err)
	}

	msg := lib.Test{ID: lib.FabridConnectivityTest, Payload: true}
	m, _ := json.Marshal(msg)
	fmt.Println(m)

	_,err = conn.Write(m)
	if err != nil{
		return fmt.Errorf("couldn't write the message : %w", err)
	}

	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil{
		return fmt.Errorf("couldn't read the message : %w", err)
	}
	if n > 2048{
		return fmt.Errorf("message longer than buffer")
	}

	var res lib.TestResult
	if err := json.Unmarshal(buf[:n], &res); err != nil{
		fmt.Println("Raw reply:", string(buf[:n]))
		return err
	}
	fmt.Printf("Verifier Replied: ID=%d State=%s \n", res.ID, res.State)

	return nil
}

type ValidFABRIDPath struct{
	Path snet.Path
	Hops []snet.HopInterface
	matches bool
	ml *fabridquery.MatchList
}

// sortValidFABRIDPaths sorts ValidFABRIDPath slice by path length (shortest first),
// then by interface IDs lexicographically, without removing any elements.
func sortValidFABRIDPaths(fabridPaths []ValidFABRIDPath) []ValidFABRIDPath {
    if len(fabridPaths) <= 1 {
        return fabridPaths
    }

    sort.Slice(fabridPaths, func(i, j int) bool {
        pathI := fabridPaths[i].Path
        pathJ := fabridPaths[j].Path
        
        metaI := pathI.Metadata()
        metaJ := pathJ.Metadata()
        
        // Handle nil metadata
        if metaI == nil && metaJ != nil {
            return false
        }
        if metaI != nil && metaJ == nil {
            return true
        }
        if metaI == nil && metaJ == nil {
            return false
        }
        
        // Compare by number of interfaces first (shorter is better)
        lenI := len(metaI.Interfaces)
        lenJ := len(metaJ.Interfaces)
        
        if lenI != lenJ {
            return lenI < lenJ
        }
        
        // Same length - compare by interface IDs
        return comparePathsByInterfaceIDs(pathI, pathJ) < 0
    })

    return fabridPaths
}

func checkFabrid(exp fabridquery.Expressions, paths []snet.Path, hops [][]snet.HopInterface) []ValidFABRIDPath{
	var res []ValidFABRIDPath

	for i, path := range paths{
		hop := path.Metadata().Hops()
		var ml *fabridquery.MatchList
		ml = &fabridquery.MatchList{
			SelectedPolicies: make([]*fabridquery.Policy, len(hop)),
		}
		b, ml := exp.Evaluate(hop, ml)

		good := true
		fmt.Printf("boolean from evaluate : %t\n", b)
		for i, pol := range ml.SelectedPolicies{
			if pol == nil || pol.String() == "reject" {
				good =false

			}
			fmt.Printf("Good on %d element : %t\n", i, good)
		}
		
		if b && good{
			res = append(res, ValidFABRIDPath{Path: path, Hops: hop, matches: b, ml: ml})
			fmt.Printf("After checking path %d : %v\n", i, res)
		}
	}

	return res
	
}


func testFabridPolicy(sd daemon.Connector, ctx context.Context, localIA addr.IA, scionNet snet.SCIONNetwork, listen *net.UDPAddr, in string, testId lib.TestID) error{

	paths, err := findPath(sd, ctx, localIA, testId)
	fmt.Println("Lengths of paths : ", len(paths))
	if err != nil{
		return fmt.Errorf("couldn't fetch paths : %w", err)
	}

	var hops [][]snet.HopInterface

	for i, path := range paths{
		hops = append(hops, path.Metadata().Hops())
		fmt.Printf("Fabrid Info for Path %d\nEnabled: ", i)
		fabInfo := path.Metadata().FabridInfo
		for _, f := range fabInfo{
			fmt.Printf("%t, ", f.Enabled)
		}
		fmt.Println("")
		fmt.Printf("Policies : ")
		for _, f := range fabInfo{
			fmt.Println(f.Policies)
		}
		fmt.Println("Hops : ", hops[i])
	}
	paths = checkFabridEnabled(paths, hops)
	if len(paths) == 0{
		return fmt.Errorf("none of the Paths support Fabrid")
	}
	fmt.Println("Length of Paths after check Fabrid enabled : ", len(paths))


	fmt.Println("Length of paths after find shortest path : ", len(paths))

	input := in
	exp, err := fabridquery.ParseFabridQuery(input)
	if err !=nil{
		return fmt.Errorf("couldn't parse the FABRID query string: %w", err)
	}

	fabridStruct := checkFabrid(exp, paths, hops)

	fabridStruct = sortValidFABRIDPaths(fabridStruct)
	var chosenFabrid ValidFABRIDPath

	var p snet.Path

	validP := len(fabridStruct) != 0

	if !validP{
		paths = findShortestPaths(paths)
		selectedHops := paths[0].Metadata().Hops()
		var ml *fabridquery.MatchList
		ml = &fabridquery.MatchList{
			SelectedPolicies: make([]*fabridquery.Policy, len(selectedHops)),
		}

		_, ml = exp.Evaluate(selectedHops, ml)

		chosenFabrid = ValidFABRIDPath{Path: paths[0], Hops: selectedHops, matches: false, ml: ml}
		p = chosenFabrid.Path
	}else{
		chosenFabrid = fabridStruct[0]
		p = chosenFabrid.Path
	}
	fmt.Println("Matches : ", chosenFabrid.matches)
	fmt.Println("Chosen Path : ", p)
	fmt.Println("Selected Hops : ", chosenFabrid.Hops)
	fmt.Println("MatchList : ", chosenFabrid.ml)

	policyIDs := chosenFabrid.ml.Policies()

	scionDP, ok := p.Dataplane().(path.SCION)
	if !ok{
		return fmt.Errorf("path is not a SCION dataplane path")
	}

	fabridConfig := &path.FabridConfig{
		LocalIA: localIA,
		LocalAddr: local,
		DestinationIA: remote.IA,
		DestinationAddr: remote.Host.IP.String(),
	}
	
	fabridDPP, err := path.NewFABRIDDataplanePath(
		scionDP,
		chosenFabrid.Hops,
		policyIDs,
		fabridConfig,
		sd.FabridKeys,
	)



	
	if err != nil{
		return fmt.Errorf("couldn't create FABRID dataplane path : %w", err)
	}

	remote.Path = fabridDPP
	remote.NextHop = p.UnderlayNextHop()

	conn, err := scionNet.Dial(ctx, "udp", listen, &remote)
	if err != nil{
		return fmt.Errorf("could not establish a connection : %w", err)
	}


	msg := lib.Test{ID: testId, Payload: chosenFabrid.matches}
	m, _ := json.Marshal(msg)
	fmt.Println(m)



	_,err = conn.Write(m)
	if err != nil{
		return fmt.Errorf("couldn't write the message : %w", err)
	}


	buf := make([]byte, 4096)
	
	n, err := conn.Read(buf)

	if err != nil{
		return fmt.Errorf("couldn't read the message : %w", err)
	}


	if n > 2048{
		return fmt.Errorf("message longer than buffer")
	}

	var res lib.TestResult
	if err := json.Unmarshal(buf[:n], &res); err != nil{
		fmt.Println("Raw reply:", string(buf[:n]))
		return err
	}
	fmt.Printf("Verifier Replied: ID=%d State=%s \n", res.ID, res.State)

	fmt.Println("CHECK")

	return nil
}


func realMain() error {
	// Your code starts here.
	fmt.Println("STAAAART")
	fmt.Println("local : ", local)

	sd, err := connectDaemon(local)
	if err != nil{
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer sd.Close()

	ctx := context.Background()
	localIA, err := sd.LocalIA(ctx)
	if err != nil{
		return fmt.Errorf("could not get the local IA: %w", err)
	}
	fmt.Println("Local IA : ", localIA)

	scionNet := snet.SCIONNetwork{Topology: sd}
	lip, err := netip.ParseAddr(local)
	if err != nil {
		return fmt.Errorf("parse --local: %w", err)
	}
	listen := &net.UDPAddr{IP: lip.AsSlice(), Port: 0}

	fmt.Println("===================== TEST 01 =====================")

	err = test01(sd, ctx, localIA, scionNet, listen)
	if err != nil {
		return fmt.Errorf("test 01 failed: %w", err)
	}

	fmt.Println("===================== TEST 02 =====================")

	err = test02(sd, ctx, localIA, scionNet, listen)
	if err != nil{
		return fmt.Errorf("test 02 failed: %w", err)
	}
	
	fmt.Println("===================== TEST 10 =====================")

	err = test10(sd, ctx, localIA, scionNet, listen)
	if err != nil{
		return fmt.Errorf("test 10 failed: %w", err)
	}

	fmt.Println("===================== TEST 11 =====================")

	err = test11(sd, ctx, localIA, scionNet, listen)
	if err != nil{
		return fmt.Errorf("test 11 failed: %w", err)
	}

	fmt.Println("===================== TEST 20 =====================")

	err = test20(sd, ctx, localIA, scionNet, listen)
	if err !=  nil{
		return fmt.Errorf("test 20 failed: %w", err)
	}

	fmt.Println("===================== TEST 30 =====================")

	err = test30(sd, ctx, localIA, scionNet, listen)
	if err != nil{
		return fmt.Errorf("test 30 failed : %w", err)
	}

	fmt.Println("===================== TEST 31 =====================")

	input := "(0-0#0,0@L1000 + {0-0#0,0@L1001 ? 0-0#0,0@L1001 : 0-0#0,0@REJECT} + 0-0#0,0@REJECT)"
	err = testFabridPolicy(sd, ctx, localIA, scionNet, listen, input, lib.FabridPolicy1Test)
	if err !=nil{
		return fmt.Errorf("test 31 failed : %w", err)
	}

	fmt.Println("===================== TEST 32 =====================")

	input = "({1-0#0,0@L1000 ? 1-0#0,0@L1000 : 1-0#0,0@REJECT} + {2-0#0,0@L1001 ? 2-0#0,0@L1001 : 2-0#0,0@REJECT} + {2-0#0,0@L1002 ? 2-0#0,0@L1002 : 2-0#0,0@REJECT} + 0-0#0,0@REJECT)"
	err = testFabridPolicy(sd, ctx, localIA, scionNet, listen, input, lib.FabridPolicy2Test)
	if err != nil{
		return fmt.Errorf("test 32 failed : %w", err)
	}


	fmt.Println("===================== TEST 33 =====================")

	

	return nil
}
