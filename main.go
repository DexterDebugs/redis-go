package main

import (
	"fmt"
	"net"
	"strings"
)
//-------------------------------------SERVER--------------------------------------------------
func handleConnection(conn net.Conn){
	defer conn.Close()	//works after surrounding functions are executed

	buffer := make([]byte, 1024)	//create a buffer of 1024 bytes

	for {
		n, err := conn.Read(buffer)		//Reads that data buffer 
		if err != nil{		//error found - disconnect immediately
			fmt.Println("Client disconnected")
			return	
		}
	
		msg := string(buffer[:n])	//converts bytes into readable data
		msg = strings.ToUpper(msg)
		fmt.Println("Received: ", msg)
		if strings.Contains(msg, "PING"){
			conn.Write([]byte("+PONG\r\n"))//echoes back to the client(to send to the client terminal)
		}else{
			conn.Write([]byte("-ERR unknown command"))
		}
	}
}

func main(){
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