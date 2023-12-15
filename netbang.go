/******************************************************************************
* netBang
*
*
* Scrappy network scanner written in Go, mostly to answer what boost Go
* concurrency gives. Also, fun to see how far I can get network-features-wise.
*
* Props to Fyodor =) Nmap is still and will likely remain, the boooooomb ;)
* In other words, this isn't supposed to replace or unthrone anything; maybe
* just add to a class of cool tools I have used and love.
*
* Making this up as I go, by whatever entertains me most >8]
*
* 14AUG2023
* CT Geigner ("chux0r")
*
* 12DEC2023 - Renamed to "netBang", due to the fact that it's at this time a
* noisy scanner. It is pretty fast though so there's that. I'll work on the
* stealthy bits soon enough. --ctg
*
* Things to ponder, improve, solve
* ------------------------------------------------------
* net.Dial() is pretty ok, but it abstracts lots of stuff I'd like to monitor
* or even change. Anywho, I'm stuck with a full-3-way TCP handshake since
* there's no controlling the connection or the flags or anything like that.
* NOTE: I did figure out, as I suspected, that the network error messages are
* coming from the stack in the OS, and are dependent in that way. I still want
* to parse them to do some interpretation and output to the users, but might
* have to do so in OS-specific add-ins.
*
* Next features hit-list:
* ------------------------------------------------------
* UDP scanning (DialUDP)
* Recon using Shodan data
* Connect() Flags scan configurations (TCP half open, Xmas, etc)
* Improved error processing/context-adding/reporting
*
* Ideas! Fun to watch 'em rot in a pile. Amazing when I actually implement!
* =============================================================================
* Multicast fun
* BGP fun
* DNS fun
* SSL cert eval, and validation
* IP history & "associations"
* Packet constructor
* Custom TCP flags options
* more integration using stdlib net structures and interfaces
* ICMP scanning/host ping and other ICMP uses
* Hardware address/local network tomfoolery
******************************************************************************/
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
)

type NetSpec struct {
	Protocol string   // "tcp" - expand into "ProtoSpec" later to accommodate UDP, ICMP/Type/Subtype
	PortList []uint16 // all der portz
	//Flags    net.Flags		// xmas comes early, every time; (impl. syscall.RawConn)
	//Packet 	 []byte			// packet constructor
}

type DnsData struct {
	Dns      net.Resolver // DNS fun, but mostly lookups/resolution
	Addrs    []string     // IPs resolved
	RevNames []string     // IP reverse-lookup names
}

type TargetSpec struct {
	Addr    string // names or IPs in string fmt == we should use net.Addr.String() for each element
	Ip      net.IP // []byte, it's the methods we want, really
	isIp    bool
	isHostn bool
	//Mac  net.HardwareAddr // layer 2; local net
}

type ScanSpec struct {
	Target   TargetSpec
	NetDeets NetSpec
}

// set up global constants for our port selection and use
const (
	adminPortRange uint   = 1024
	maxPorts       uint16 = 65535
)

var thisScan ScanSpec // TODO: newScanSpec constructor, returning *ScanSpec
var Resolv DnsData

func init() {
	scanConstructor() // initialize our struct with reasonable default values

	// TODO: complete flags/options commented out below:
	//doDo       := flag.Bool("do", false, "Specify the activity: scan, qrecon, dnsinfo. Default is \"scan\".")
	//fakeDo     := flag.Bool("dryrun", false, "Do not execute. Print current activities list, pre-validate all, print config and pre-conditions.")
	helpDo := flag.Bool("h", false, "Pull up the detailed \"help\" screen.")
	helpDo2 := flag.Bool("help", false, "Same as \"-h\", above.")
	listsDo := flag.Bool("l", false, "Print all pre-configured TCP and UDP port group lists and list names. \n\t(--lists <Listname>) shows detailed port listing for <Listname>.")
	listsDo2 := flag.Bool("lists", false, "Same as \"-l\", above.")
	portsDo := flag.String("p", "", "Specify a port or ports, and/or named portlists to use in a comma-delimited list. TCP or UDP scans only.\n\t(Available port lists may be pulled up with \"netscanx --lists\")")
	portsDo2 := flag.String("ports", "", "Same as \"-p\", above.")
	portsfileDo := flag.String("pf", "", "Input comma-delimited list of target ports from a file.")
	portsfileDo2 := flag.String("portsfile", "", "Same as \"-p\", above.")
	protoDo := flag.Bool("proto", false, "Define the protocol to use: tcp, udp, or icmp. Default is \"tcp\".")
	dnsrvDo := flag.Bool("resolver", false, "Set DNS resolver to use. Default is to use your system's local resolver.")
	/*
		verboseDo  := flag.Bool("v", false, "Verbose runtime output")
		verboseDo2 := flag.Bool("verbose", false, "Same as \"-v\", above. ")
		verboseDo3 := flag.Bool("vv", false, "Debug-level-verbosity runtime output. Obscenely verbose.")
		verboseDo4 := flag.Bool("debug", false, "Same as \"-vv\", above.")
	*/

	flag.Parse()

	// HELP MENU
	if *helpDo || *helpDo2 || len(os.Args) <= 1 { //Launch help screen and exit
		fmt.Print(
			`
USAGE:
netbang [-h|--help]
	Print this help screen
netbang [-l|--lists] [<Listname>] 
	Print all usable pre-configured TCP and UDP port group lists and names. With <Listname>, show detailed port listing for <Listname>. 

netbang [[FLAGS] <object(,optionals)>] <TARGET>
	FLAGS
		[-p|--ports] <num0(,num1,num2,...numN,named_list)> 
		Specify port, ports, and/or named portlists to use in a scan. TCP or UDP proto only. 
		(View named portlists with --lists)

		[-pf|--portsfile] <(directory path/)filename>
		Input from file a comma-delimited list of port numbers to scan. TCP or UDP proto only.

		[--proto] <tcp|udp>
		Specify scanning protocol, tcp, udp, or icmp. Default is "tcp".
		
		[--resolver] <ipaddr> 
		DNS resolver to use. Default is to use your system's local resolver.
	
	<TARGET> 
		Object of scan. Target must be an IP address, an IP/CIDR range, or a valid 
		hostname.
	
`)
		/* On tap, but not ready yet --ctg

		[--dryrun]
			Print activities list, pre-validate targets, print config and pre-conditions.
			Dry-run does NOT execute the scan.

		[-x|--exec] <scan(,tcpscan,udpscan,dnsinfo)>
			Specify activity(s): scan, tcpscan, udpscan, or dnsinfo. Default is "scan". */

		os.Exit(0)
	} else if *listsDo != false || *listsDo2 != false {
		if flag.Arg(0) == "" {
			fmt.Print("Placeholder for list available lists\n") // TODO: list lists func
		} else {
			fmt.Print("Placeholder for per-list item printout\n") // TODO: list named list items func
		}
		os.Exit(0)
	}

	/*
		if *portsDo || *portsDo2  {
			if flag.Arg(0) == "" {
				fmt.Print("Error: No ports listed! You must list at least one port number with \"--ports\".")
				os.Exit(1)
			} else {
				thisScan.NetDeets.PortList = []uint16{}                     // clear the defaults
				pargs := flag.Arg(0)                                        // gather user-spec'd ports
				p, pl := parsePortsCdl(pargs)                               // TODO: create func that assembles final []uint16 port list from spec
				fmt.Println("Ports specified: ", p, "List specified: ", pl) // TEST/TODO: remove when port assembler complete
				if len(pl) > 0 {                                            // if we have named lists...
					for i := 0; i < len(pl); i++ {
						// resize the portlist appropriately and reassemble
						ts1 := thisScan.NetDeets.PortList
						ts2 := buildNamedPortsList(pl[i])
						thisScan.NetDeets.PortList = make([]uint16, len(ts1)+len(ts2), len(ts1)+len(ts2)+32)
						copy(thisScan.NetDeets.PortList, ts1)
						thisScan.NetDeets.PortList = append(thisScan.NetDeets.PortList, ts2...)
					}
				}
				if len(p) > 0 {
					for _, ptmp := range p {
						thisScan.NetDeets.PortList = append(thisScan.NetDeets.PortList, ptmp)
					}
				}
			}
		}
	*/
	if len(*portsDo) > 0 || len(*portsDo2) > 0 || len(*portsfileDo) > 0 || len(*portsfileDo2) > 0 { // what if some crazy person sets all of these? Hm. Sure. Why not. Just detect it and append everything together with slicefu
		thisScan.NetDeets.PortList = []uint16{}      // clear the default port definitions since we GOIN'CUSTOM yee-haw
		if len(*portsDo) > 0 && len(*portsDo2) > 0 { //ifdef -p --ports
			log.Print("Warning: Ports given with both -p and --ports. Combining.")
		}
		if len(*portsfileDo) > 0 && len(*portsfileDo2) > 0 { // -pf --portsfile <filename>
			log.Print("Warning: Multiple input files given with both -pf and --portsfile. Combining.")
		}
		if len(*portsDo) > 0 { // ifdef -p
			doPortsFinal(*portsDo)
		}
		if len(*portsDo2) > 0 { // ifdef -ports
			doPortsFinal(*portsDo2)
		}
		if len(*portsfileDo) > 0 { // ifdef -pf read given user port config file
			log.Printf("Opening user-defined port config file [%s].", *portsfileDo)
			pconf, err := os.Open(*portsfileDo)
			p := make([]byte, 4096)
			if err != nil {
				log.Fatalf("Error opening file [%s]: [%s]. Exiting", *portsfileDo, err.Error())
			}
			defer pconf.Close()
			fsize, err := pconf.Read(p)
			if err != nil {
				log.Fatalf("Error reading file: [%s]. Exiting", *portsfileDo)
			}
			p = p[:fsize] // trim buffer to infile size or we'll have NUL padding everywhere, which will cause paresePortsCdl to misparse and barf
			fmt.Printf("\nData read from cf file: >> %s", string(p))
			doPortsFinal(string(p))
		}
	}

	if *protoDo {
		if flag.Arg(0) == "" {
			fmt.Print("\nWarning: No protocol listed with --proto. Using \"tcp\".")
		} else {
			thisScan.NetDeets.Protocol = strings.ToLower(flag.Arg(0))
			if thisScan.NetDeets.Protocol != "tcp" && thisScan.NetDeets.Protocol != "udp" {
				log.Fatalf("Error: Invalid protocol: %s! Allowed protocols are \"tcp\" or \"udp\".", flag.Arg(0))
			}
		}
	}
	/*
		if *doDo {
			if flag.Arg(0) == "" {
				fmt.Print("Error: You must specify which netscanx activity to do after \"--do\" (tcpscan, udpscan, dnsinfo). Default is tcpscan.")
				os.Exit(1)
			}
		}
	*/
	if *dnsrvDo {
		if flag.Arg(0) == "" {
			log.Fatal("Error: You must specify the IP of a DNS server to use with \"--resolver\".")
		} else {
			setCustomResolver(&Resolv.Dns, flag.Arg(0)) // pass it our DnsInfo struct to populate/use
		}
	}
	// TODO: TARGET VALIDATION CODE GOES HERE
	thisScan.Target.Addr = os.Args[len(os.Args)-1] // last arg will always be the target hostname/addr
}

func main() {
	bangHost(thisScan.NetDeets.PortList, thisScan.Target.Addr, thisScan.NetDeets.Protocol)
}

/*
bangHost()

	INPUT: 	[]uint16 port list (can be empty)
			protocol, TCP or UDP
			target hostname or IP
	PROCESSING
			Launches concurrent port scans at a target host
			Catches results strings via IPC channel receiver.
	OUTPUT
			Scan results data/report
*/
func bangHost(pl []uint16, host string, proto string) {
	// TCP scan - For all ports given, scan single host and format results
	scanReport := make([]string, 0, len(pl)) // report of scan responses, usually by tcp/udp port number. Portlist len to avoid reslicing.
	scanIpc := make(chan string)             // com pipe: raw, unordered host:port try response data or errors
	rxc := 0
	job := 0
	if len(pl) <= 0 { // if no ports specified, use short default common ports
		if proto == "tcp" {
			pl = buildNamedPortsList("tcp_short")
		} else if proto == "udp" {
			pl = buildNamedPortsList("udp_short")
		} else {
			log.Fatalf("Error: Invalid protocol: [%s]! Allowed protocols are \"tcp\" or \"udp\".", proto)
			os.Exit(1)
		}
	}
	fmt.Printf("\nBangHost: [%s], Portcount: [%d]\n=====================================================", host, len(pl))

	for _, port := range pl { // For all ports given, bang each one and report results
		hp := getHostPortString(host, port)
		if proto == "tcp" {
			go bangTcpPort(hp, scanIpc, &job) // Bang bang! Single host:port per call
		} else if proto == "udp" {
			go bangUdpPort(hp, scanIpc, &job)
		} else {
			log.Fatalf("Error: Invalid protocol: [%s]! Allowed protocols are \"tcp\" or \"udp\".", proto)
		}
	}
	fmt.Println("\nPortBangers running...")

	// Channel receiver :: Get all concurrent scan job output and report
	for i := 0; i < len(pl); i++ {
		//for log := range scanIpc {
		log := <-scanIpc
		rxc++
		scanReport = append(scanReport, log)
	}
	fmt.Printf("\nJobs run: %d", job) //TEST
	//fmt.Printf("\nRecv'd job logs: %d.", rxc) //TEST
	close(scanIpc)
	printReport(scanReport)
}

/*
bangTcpPort()

	--:: [BANG, AS IN .:|BANG|:. *FUCKIN NOISY*] ::--
		Full 3-way TCP handshake
		net.Dial seems to like retrying [SYN->] sometimes (!) after getting [<-RST,ACK] lol

	Hits given target:port and records response.
	Shoots results back through IPC channel.
*/
func bangTcpPort(t string, ch chan string, job *int) {
	*job++
	joblog := fmt.Sprintf("[%s] -->\t", t)
	conn, err := net.Dial("tcp", t) // TODO: add default max time wait to this + Make configurable
	if err != nil {
		fmt.Printf("💀")
		ch <- fmt.Sprint(joblog, "[💀] ERROR: ", err.Error())
		//fmt.Printf("\n[%s]: Connection error: %s", t, err.Error())
	} else {
		defer conn.Close()
		fmt.Print("😎")
		ch <- fmt.Sprint(joblog, "[😎] OPEN")
		//fmt.Printf("\n[%s]: Connected ok: open", t)
	}
}

/*
bangUdpPort()

	What. Isup. With Datagrams, amirite?
	Hits given UDP target:port and records response.
	Shoots results back through IPC channel.
*/
func bangUdpPort(t string, ch chan string, job *int) {
	rcvbuf := make([]byte, 1024)
	*job++
	joblog := fmt.Sprintf("[%s] -->\t", t)
	udpaddr, err := net.ResolveUDPAddr("udp", t)
	conn, err := net.DialUDP("udp", nil, udpaddr) // TODO: add default max time wait to this + Make configurable
	if err != nil {
		fmt.Printf("💀")
		ch <- fmt.Sprint(joblog, "[💀] ERROR: ", err.Error())
		//fmt.Printf("\n[%s]: Connection error: %s", t, err.Error())
	} else {
		defer conn.Close()
		_, err = conn.Write([]byte("loludp"))
		if err != nil {
			fmt.Printf("💀")
			ch <- fmt.Sprint(joblog, "[💀] ERROR: ", err.Error())
		} else {
			_, err = conn.Read(rcvbuf)
			if err != nil {
				fmt.Printf("💀")
				ch <- fmt.Sprint(joblog, "[💀] ERROR: ", err.Error())
			} else {
				fmt.Print("😎")
				ch <- fmt.Sprint(joblog, "[😎] OPEN")
			}
		}
	}
}

func printReport(ss []string) {
	fmt.Printf("\n%s Scan Results\n================================================================================", thisScan.Target.Addr)
	for _, result := range ss {
		fmt.Printf("\n%s", result)
	}
	fmt.Print("\n")
}
