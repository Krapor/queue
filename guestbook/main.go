
http.Handle("/", http.FileServer(http.Dir("./static")))
