# sparkyfish
An open-source internet speed and latency tester.  You can test your bandwidth against a public Sparkyfish server or host your own server with the included server-side daemon.

<img src="http://island.nu/github/sparkyfish/sparkyfish-v1.1.png">

# About

Sparkyfish offers several advantages over speedtest.net and its unofficial clients like [speedtest-cli](https://github.com/sivel/speedtest-cli)

* You can run your own **private** sparkyfish server, free-of-charge. (compare to $1,995+ that Ookla charges for a private speedtest.net server)
* You can test speeds > 1 Gbps if your server and client hosts support them.  Most speedtest.net test servers don't have this capacity.
* Sparkyfish comes with a colorful console-based client that runs on *nix and Windows
* [No net-neutrality issues](https://www.techdirt.com/blog/netneutrality/articles/20141124/14064729242/fcc-gives-t-mobile-talking-to-exempting-speedtests-caps-preventing-users-seeing-theyd-been-throttled.shtml)
* Sparkyfish uses an open protocol for testing.  You're welcome to implement your own alternative front-end client or server!

# Sparkyfish CLI
## Installation
The easiest way to get started is to [download a binary release](https://github.com/chrissnell/sparkyfish/releases/).  Sparkyfish is written in Go and compiles to a static binary so there are no dependencies if you're using the official binaries.  

Once you've downloaded the binary...
```
gunzip <binary>.gz
chmod 755 <binary>
mv <binary> /usr/local/bin/sparkyfish-cli
```

## Running the client
Run the client like this:

```sparkyfish-cli <sparkyfish server IP>[:port]```

The client takes only one parameter.  The IP (with optional :port) of the sparkyfish server.  You can use our public server round-robin to try it out:  ```us.sparkyfish.chrissnell.com```.  Sparkyfish servers default to port 7121.

For more info on the public sparkyfish servers, see [docs/PUBLIC-SERVERS.md](docs/PUBLIC-SERVERS.md).

## Running from Docker (optional)
You can also run ```sparkyfish-cli``` via Docker.  I'm not sure if this is the most optimal way to use it, however. After running the client once, the terminal window environment gets a little hosed up and sparkyfish-cli will complain about window size the next time you run it.  You can fix these by running ```reset``` in your terminal and then-re-running the image.

If you want to test it out, here's how to do it:

```
docker pull chrissnell/sparkyfish-cli:latest
docker run --dns 8.8.8.8 -t -i chrissnell/sparkyfish-cli:latest us.sparkyfish.chrissnell.com
reset  # Fix the broken terminal size env before you run it again
```

## Building from source (optional)
If you prefer to build from source, you'll need a working Go environment (v1.5+ recommended) with ```GOROOT``` and ```GOPATH``` env variables properly configured.   To build from source, run this command:

```
go get github.com/chrissnell/sparkyfish
```

Your binaries will be placed in ```$GOPATH/bin/```.

# Running your own Sparkyfish server
## Running from command line
You can download the latest ```sparkyfish-server``` release from the [Releases](https://github.com/chrissnell/sparkyfish/releases/) page.  Then:
```
gunzip <binary filename>.gz
chmod 755 <binary filename>
./<binary filename> -h  # to see options
./<binary filename> -location="Your Physical Location, Somewhere"
```

By default, the server listens on port 7121, so make sure that you open a firewall hole for it if needed.  If the port is firewalled, the client will hang during the ping testing.

## Docker method
```
docker pull chrissnell/sparkyfish-server:latest
docker run -e LOCATION="My Town, Somewhere, USA" -d -p 7121:7121 chrissnell/sparkyfish-server:latest
```
