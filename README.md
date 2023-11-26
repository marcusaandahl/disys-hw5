# disys-hw5 (Hand-in 5: A distributed auction system)

### _Team Bastian_
## Starting the program

Run the server file with
```bash
go run ./server/server.go
```
When running the server file, the server will try to initialize on `localhost:3000`. If this is successful, it will take on the role of primary server. If port 3000 is occupied (by another server instance), the server will use 3001 and acts as a backup server.

Run the client file with

```bash
go run ./client/client.go
```
A client will automatically attempt to connect to localhost:3000 (ie. the primary server). If this connection fails it will attempt port 3001 (as this could be due to a client joining after a crash of the primary server).

## Commands - the auction system

A client may start an auction with `bid x`, where x is an integer amount. When an auction is started, it runs for 100 seconds.
A client may query the state of an auction with `result`, which will return the current highest amount bidded or a statement indicating the auction is over.

__Note:__ For ease of using the program, we have allowed continous initiation of auctions. That is, a client may write `bid XXX` after another auction is over -> this will initiate a new auction instead of being denied. The user will be informed that a new auction has been started and that the bid is not valid for the previous auction. We have implemented this as it is written in the project description, in the real world it would make sense to inform all users automatically that an auction has ended.