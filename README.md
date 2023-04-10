# go-websockets

## Overview

The server side, written in go, consists of 3 main components: a web server, 
an authentication service, and a messages service. The web server handles
incoming http and websocket requests, parses and validates request data,
and passes requests along to either the auth or message service.

### Building & Running
In order to build the client, you must have node installed. In the `/client`
directory, run `npm install` and then `npm run build`. Then, run the server
with either `go run . [port] [password]` (or build and then run). The client
will be accessible at `http://localhost:[port]`.

### Tests
The most important test is the test of the messages service, in 
`/msg/messages_test.go`. In the test, 1000 users subscribe to the
service and simultaneously send 1 message each. The test verifies
that all 1000 users each receive exactly one copy of each of the 1000
messages sent.

I discovered several deadlock conditions with this test that don't appear
in normal use.

### Web Server
Endpoints:
- `POST /login`: log in with the server password and a display name.
    Receive a token.
- `GET /history?page=[int]`: paginated chat history endpoint. 
    Requires token in header.
- `GET /join?token=[string]`: endpoint that upgrades connection to websocket
    connection upon validation of the token.
- `GET /logout`: removes the user's token from the auth service

The web server passes messages to the auth and message service to retrieve
or create the requested resources.

### Message service
The message service is a goroutine which handles incoming
messages and paged requests for message history. It stores the server message history
in a linked list of buckets, each of which holds 100 messages. A bucket represents
a page of message history that can be returned by the `/history` endpoint. Each bucket
has a number which increases as buckets are added. The list does not need to be traversed
unless a user wants more than 100 messages of history. Access to message storage, both
to add new messages and to respond to message history requests, is synchronized by
channels.

### Auth service
The auth service is a goroutine which handles logins and authentication of websocket
requests. If a user provides the correct server password, they receive a token
which can be used to join the chat room and request the message history. When validating
tokens, it also checks whether the token has expired and deletes it if so. Access to the
map of users, which is used by the message service to broadcast messages to all users,
is synchronized by a RWLock mutex.
