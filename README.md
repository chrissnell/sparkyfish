# sparkyfish
An open-source internet speed and latency tester in client/server form. 

<img src="http://island.nu/github/sparkycon/screenshot-1.1.png">

# About

Sparkyfish offers several advantages over speedtest.net and its unofficial clients like [speedtest-cli](https://github.com/sivel/speedtest-cli)

* You can run your own private sparkyfish server, free-of-charge. (compare to $1,995+ that Ookla charges for a private speedtest.net server)
* You can test speeds > 1 Gbps if your server and client hosts support them.  Most speedtest.net test servers don't have this capacity.
* Sparkyfish comes with a colorful console-based client that runs on *nix and Windows
* Sparkyfish results are less likely to be influenced by ISP's traffic prioritization.  ISPs have been known to give speedtest.net traffic priority in order to artificially improve their users' test results.  [Even though T-Mobile may be throttling your network connection, they purposely don't throttle speedtest.net](https://www.techdirt.com/blog/netneutrality/articles/20141124/14064729242/fcc-gives-t-mobile-talking-to-exempting-speedtests-caps-preventing-users-seeing-theyd-been-throttled.shtml), which may be giving you an inaccurate view of your actual speeds.  Since sparkyfish uses its own proprietary ports and tests with randomly-generated data, it's less likely to be throttled/unthrottled (for now).
* Sparkyfish uses an open protocol for testing.  You're welcome to implement your own front-end client!

# Sparkyfish CLI
## Installation
The easiest way to get started is to [download a binary release](https://github.com/chrissnell/sparkyfish/releases/).  Sparkyfish is written in Go and compiles to a static binary so there are no dependencies if you're using the official binaries.  

Once you've downloaded the binary, unzip it and move it to somewhere in your $PATH, like ```/usr/local/bin/sparkyfish-cli```

## Running the client
Run the client like this:

```sparkyfish-cli <sparkyfish IP:port>```

The client takes only one parameter.  The IP:port of the sparkyfish server.  You can use our public server round-robin to try it out:  ```us.sparkyfish.chrissnell.com:7121```

For more info on the public sparkyfish servers, see [docs/PUBLIC-SERVERS.md](docs/PUBLIC-SERVERS.md).

## Building from source (optional)
If you prefer to build from source, you'll need a working Go environment (v1.5+ recommended) with ```GOROOT``` and ```GOPATH``` env variables properly configured.   To build from source, run this command:

```
go get -t github.com/chrissnell/sparkyfish
```

Your binaries will be placed in ```$GOPATH/bin/```.

