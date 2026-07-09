package main

import (
    "fmt"
    "os"
)

func main() {
    fmt.Println("easysearch: skeleton boot ok")
    if len(os.Args) > 1 && os.Args[1] == "--version" {
        fmt.Println("0.1.0")
        return
    }
}
