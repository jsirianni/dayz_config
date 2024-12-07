package main

import (
    "bytes"
    "fmt"
    "net"
    "time"
)

func main() {
    serverAddr := "50.108.116.1:2324"
    conn, err := net.Dial("udp", serverAddr)
    if err != nil {
        panic(err)
    }
    defer conn.Close()

    // Query packet: 0xFF 0xFF 0xFF 0xFF TSource Engine Query
    query := []byte{0xFF, 0xFF, 0xFF, 0xFF, 'T', 'S', 'o', 'u', 'r', 'c', 'e', ' ', 'E', 'n', 'g', 'i', 'n', 'e', ' ', 'Q', 'u', 'e', 'r', 'y', 0x00}
    conn.SetDeadline(time.Now().Add(3 * time.Second))
    _, err = conn.Write(query)
    if err != nil {
        panic(err)
    }

    buffer := make([]byte, 2048)
    n, _, err := conn.ReadFrom(buffer)
    if err != nil {
        panic(err)
    }

    fmt.Println("Server Response:")
    fmt.Println(bytes.Trim(buffer[:n], "\x00"))
}

