# ws-subprocess

[![Build Status](https://api.travis-ci.org/bwalex/ws-subprocess.svg?branch=master)](https://travis-ci.org/bwalex/ws-subprocess)

ws-subprocess is a HTTP/websocket server that can run a (verified) command as a process and attach its stdin/stdout/stderr to a websocket connection.


## Get it

Release binaries for linux amd64 platforms are built by default and can be downloaded from the [Releases page](https://github.com/bwalex/ws-subprocess/releases).

For other platforms or to build from source, clone the repository and just run `make`.


## Usage

Usage of dist/ws-subprocess:
  -controller-url string
    	URL of controller (default "http://127.0.0.1/ws-controller")
  -listen string
    	Listen address (default "127.0.0.1:8866")


## Details

When ws-subprocess is started, it will listen on the IP and port specified by the `-listen` flag for HTTP/websocket requests to the `/ws` endpoint. Requests to this endpoint should include a `token` query parameter.

When a request is received on the `/ws` endpoint, ws-subprocess will send a GET request to the controller (at the URL specified by the `-controller-url` flag) including the same `token` query parameter it received.

The token allows to maintain some state between the controller and the client. This way, the controller can handle authentication and making sure only specific (allowed) commands get executed. An example use case would be for the controller to allow logged in users to run some specific command by:

 1) generating a long, random token
 2) inserting an entry into a database containing the token and some command to run
 3) rendering a page for the user containing logic to initiate a websocket connection to a ws-subprocess with the given token
 4) ws-subprocess receives the request and provides the token back to the controller
 5) the controller validates the token (e.g. by checking that it is in the database) and provides a response indicating which command to run (e.g. also taken from the database entry)
 6) ws-subprocess runs the given command and connects the user's websocket to it

Another possibility is to use JSON Web Tokens (JWT) for tokens to avoid storing any state on the server side. Either way, for ws-subprocess the token is just an opaque value.

If the controller responds with any status code other than 200, an error response is sent to the client and the websocket connection/upgrade is aborted (and no command is run).

If the controller responds with a status 200 HTTP OK response, then ws-subprocess expects a JSON payload in the body with the following schema, where the command must be an absolute path to the command to run:

    {
      command: string,
      args: []string
    }

For example:

    {"args": ["/etc/resolv.conf"], "command": "/usr/bin/nano"}

ws-subprocess will then run the command with the given arguments in a new process, and connect stdin, stdout and stderr directly to the upgraded websocket.


## Running the example

Install dependencies of the example server/controller:

    cd example
    pip install -r requirements.txt

Run the example server (within the `example/` directory) passing command that should be run when a client connects as argument(s), for example:

    ./web.py /usr/bin/nano /etc/resolv.conf

Run `ws-subprocess` in another shell:

    ws-subprocess -controller-url http://127.0.0.1:5000/ws-controller

Open a browser and point it at http://127.0.0.1:5000 - you should see an xterm.js window with the process visible within it.


## License

ws-subprocess is released under the [MIT License](http://www.opensource.org/licenses/MIT).
