package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)
var store = map[string]string{}		//collection of key-value pairs
var expiry = map[string]time.Time{}	//setting expiry time for each pair
var mu sync.Mutex

//-------------------------------------SERVER--------------------------------------------------
func handleConnection(conn net.Conn){
	defer conn.Close()	//works after surrounding functions are executed

	reader := bufio.NewReader(conn) 	//create a bufio wrapping conn

	for {
		cmd, err := readCommand(reader)		//parse one cmd
		if err != nil{		//error found - disconnect immediately
			fmt.Println("Client disconnected")
			return	
		}
		if len(cmd) == 0{
			continue
		}
		name := strings.ToUpper(cmd[0])
		switch name {
		case "PING":
				conn.Write([]byte("+PONG\r\n"))
		case "SET":
			if len(cmd) < 3{
				conn.Write([]byte("-ERR wrong number of arguments\r\n"))
				continue
			}
			mu.Lock()
			store[cmd[1]] = cmd[2]	//In the map called store, set the key "name" to have the value "Alex"
			mu.Unlock()
			conn.Write([]byte("+OK\r\n"))
		case "GET":
			if len(cmd) < 2{
				conn.Write([]byte("-ERR wrong number of arguments\r\n"))
				continue
			}
			mu.Lock()
			isExpired(cmd[1])		// clean up if expired (we hold the lock)
			value, exists := store[cmd[1]]	// now check — expired key is already gone
			mu.Unlock()
			if exists{
				reply := fmt.Sprintf("$%d\r\n%s\r\n", len(value), value)
				conn.Write([]byte(reply))
			}	else {
				conn.Write([]byte("$-1\r\n"))
			}
		case "DEL":
			if len(cmd) < 2{
				conn.Write([]byte("-ERR wrong number of arguments\r\n"))
				continue
			}
			mu.Lock()
			_ , exists := store[cmd[1]]	//check if key exists
			delete(store, cmd[1])
			mu.Unlock()
			if exists{
				conn.Write([]byte(":1\r\n"))
			}	else {
				conn.Write([]byte(":0\r\n"))
			}
		case "EXISTS":
			if len(cmd) < 2{
				conn.Write([]byte("-ERR wrong number of arguments\r\n"))
				continue
			}
			mu.Lock()
			_, exists := store[cmd[1]]
			mu.Unlock()
			if exists{
				conn.Write([]byte(":1\r\n"))
			}	else {
				conn.Write([]byte(":0\r\n"))
			}
		case "EXPIRE":
			if len(cmd) < 3{
				conn.Write([]byte("-ERR wrong number of arguments\r\n"))
				continue
			}
			secs, err := strconv.Atoi(cmd[2])
			if err != nil {	
				conn.Write([]byte("-ERR value is not an integer\r\n"))
				continue	
			}
			mu.Lock()
			_, exists := store[cmd[1]]
			if exists{
				expiry[cmd[1]] = time.Now().Add(time.Duration(secs) * time.Second)
				//Debugging purpose:
				//fmt.Println("DEBUG EXPIRE: key=", cmd[1], "secs=", secs, "expiry set to=", expiry[cmd[1]], "now=", time.Now())
				//time.Duration(secs) * time.Second — turns the int secs into a duration (e.g. 60 * time.Second)
				//time.Now().Add(d) — a moment d into the future
			}
			mu.Unlock()
			if exists {
				conn.Write([]byte(":1\r\n"))
			} else {
				conn.Write([]byte(":0\r\n"))
			}
		case "TTL":
			if len(cmd) < 2{
				conn.Write([]byte("-ERR wrong number of arguments\r\n"))
				continue
			}
			mu.Lock()
			isExpired(cmd[1])
			_, exists := store[cmd[1]]
			exp, hasExpiry := expiry[cmd[1]]
			mu.Unlock()
			if !exists {
				conn.Write([]byte(":-2\r\n"))		//key doesn't exist
			}	else if !hasExpiry{					//exists but never expires
				conn.Write([]byte(":-1\r\n"))
			}	else {
				remaining := int(time.Until(exp).Seconds())
				//time.Until(exp) — duration from now until exp;
				//  .Seconds() gives a float, wrap in int(...) for whole seconds
				reply := fmt.Sprintf(":%d\r\n", remaining)   // capture it
 				conn.Write([]byte(reply))			//send it
			}
		default:
				conn.Write([]byte("-ERR unknown command\r\n"))
		}
		fmt.Println("Received command:", cmd)
	}
}
//----------------------------------------EXPIRATION FUNCTION------------------------------------------------------------
func isExpired(key string) bool {
	exp, hasExpiry := expiry[key]
	if !hasExpiry	{
		return false
	}
	if time.Now().After(exp){	//returns true if the current moment is later than the expiry time (i.e., the deadline has passed)
		delete(store, key)
		delete(expiry, key)
		return true	// was expired, now deleted
	}
	return false	// has expiry, but not reached yet
}

//----------------------------------------PARSER FUNCTION---------------------------------------------------------
func readCommand(reader *bufio.Reader) ([] string, error){	//takes raw protocol bytes off a wire and turns them into a clean, usable data structure
	//read the array header	- "*3\r\n"
	line, err := reader.ReadString('\n')	//returns string and error
	if (err != nil){
		return nil, err
	}

	//Strip the '*' and the trailing \r\n
	line = strings.TrimSpace(line)		//you are FORGETTING to return function values by re-assigning them

	if len(line) == 0 || line[0] != '*'{		//if first char is not *, return error
		return nil,fmt.Errorf("expected '*', got %q", line)
	}
	n, err := strconv.Atoi(line[1:])		//line[1:] returns a string
	if (err != nil){				//atoi takes string as input, returns int, err as output
		return nil, err
	}
	results := make([]string, 0, n)		//make string array with 0 length, n capacity
	for i := 0; i < n; i++ {	//read 'n' bulk strings
		header, err := reader.ReadString('\n')	//read "$3\r\n"
		if err != nil{
			return nil, err
		}
		header = strings.TrimSpace(header)	//"$3"

		if len(header) == 0 || header[0] != '$'{	//check if starts with '$'
			return nil, fmt.Errorf("expected '$', got '%q'", header)
		}
		length, err:= strconv.Atoi(header[1:])
		if err != nil {
			return nil, err
		}

		buf := make([]byte, length)
		_, err = io.ReadFull(reader, buf)
		if err != nil {
			return nil, err
		}
		reader.Discard(2)
		results = append(results, string(buf))
	}
	return results, nil
}
//----------------------------TESTING PARSER-----------------------------------------------
/*func testParser() {
	raw := "*3\r\n$3\r\nSET\r\n$4\r\nname\r\n$4\r\nAlex\r\n"
	reader := bufio.NewReader(strings.NewReader(raw))
	cmd, err := readCommand(reader)
	if err != nil {
		fmt.Println("error: ", err)
		return
	}
	fmt.Println("parsed: ", cmd)
}*/

func main(){
	//testParser()	used for testing purposes

	ln,err := net.Listen("tcp", ":6379")	//ln is a listener object
	if err != nil {		//if error occurs, print it to the console
		fmt.Println("error: ", err)
		return	//end it
	}
	fmt.Println("Server running on port: 6379")
	for {						
		conn, err := ln.Accept()	//constantly keep waiting for clients
		if err != nil{	
			fmt.Println("error: ", err)
			return
		}
		go handleConnection(conn)	//hand the connection to handler
	}
}