package main

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"io"
	"strings"
)

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
		default:
				conn.Write([]byte("-ERR unknown command\r\n"))
		}
		fmt.Println("Received command:", cmd)
	}
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