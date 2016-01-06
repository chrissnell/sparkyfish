# The Sparkyfish Protocol
Sparkyfish uses a simple TCP-based client-server protocol to perform all testing.   The client connects to the server, runs a test, then disconnects.  This process is repeated for each of the three tests: ping, download, and upload.    Thus, it takes three connection in series to complete a ping+download+upload test sequence.  These tests could be conducted in parallel--there's no server-side prohibition against this--but it might render the results inaccurate.

### Protocol versioning.
The protocol is versioned but currently there is only one version: ```0```.  The client requests a certain version as part of the HELO sequence described below.

### Protocol Sequence
```client>>>``` is used to show commands sent by the client

```server<<<``` is used to show responses sent by the server

### Signing on to the server - HELO
Every test begins with a client connection to the server, followed by a ```HELO``` command sent by the client.

1. The client connects over TCP to the Sparkyfish server, typically running on port 7121.
2. Once the connection is opened, the client issues a ```HELO``` command, suffixed by the protocol version in use (integer, 0-9):
```
client>>> HELO0<newline>     # Note: that's a zero after the HELO, which signifies protocol version 0.  It's followed by a \n (newline)
server<<< HELO<newline>
server<<< my.canonical.hostname.com<newline>
server<<< My Location, Some Country<newline>
```
**Important note: The version number that follows ```HELO``` is not optional and must be sent or the server will rejection the connection.**

3. Once the HELO has completed, the server is ready for a testing command.  The client sends the command, followed by a <newline>:
```
client>>> ECO    # ECO is the command that requests an echo (ping) test
server<<< <begins echo test>
```

### Echo (Ping) test
The ping test isn't actually an ICMP ping test at all.  It's a simple TCP echo.  The client requests an echo test with the commend ```ECO``` and then sends one character at a time (***no newline***).  As soon as the server receives the client's character, it echoes it back (again, no newline is sent).  This continues for up to 30 characters (configurable on server-side) or until the client closes the connection.  If the client has not disconnected, the server will close the test after 30 characters are echoed back. to the client.

Example:
```
client>>> ECO<newline>
client>>> a
server<<< a
client>>> b
server<<< b
client>>> c
server<<< c
client>>> d
server<<< d
[... and so on ...]
```
Note that you don't have to send any particular character in the ECO test.  ```sparkyfish-cli``` sends a zero (0) every time.

### Download test
The client initiates a server->client download test with the ```SND``` command.  The download test consists of a stream of randomly-generated data, sent from the server to the client as fast as the server can send it and the client can accept it.  It is up to the client to measure the speed at which the stream is downloaded and report this back to the user.  The test continues for a *fixed time period*.  The goal is for the client to download as much data as possible within this time period, which defaults to 10 seconds.  After 10 seconds has elapsed, the server will close the connection.

Example:
```
client>>> SND<newline>
client>>>[A stream of random data is sent to the server for 10 seconds]
[ ... server closes the connection after 10 seconds of receiving ...]
```

### Upload test
The client initiates a client->server upload test with the ```RCV``` command. The upload test consists of a stream of randomly-generated data, sent from the client to the server as fast as the client can send it and the server can accept it.  It is up to the client to measure the speed at which the stream is uploaded and report this back to the user.  The test continues for a *fixed time period*.  The goal is for the client to send as much data as possible within this time period, which defaults to 10 seconds.  After 10 seconds has elapsed, the server will close the connection.  **It is up to the client to generate the random data**, though the server does not enforce what the stream actually contains.  Random data is recommended to reduce the potential for external compression.


Example:
```
client>>> RCV<newline>
server<<< [A stream of random data is sent for 10 seconds]
[ ... server closes the connection after 10 seconds of sending ...]
```
