# Assignment4
To run the program, you must clone the assignment.
After, you must navigate to \client and run client.go
For the program to function as intended you must run 3 instances of client.go with the argument "0" to instantiate the first peer, and then for the other two peers: "1" and "2"
(Example: "go run client.go 0")
This will create the three peers as proposed.
We assume port 5000 on your pc is not in use, since the program depends on it.

If you wish to test the solution with more peers, you must change the variable "numberOfClients" in client.go, and make sure to run the same number of application instances, with first argument "0", then "1", and so on.
