# 🔥 GraphQL Authorization Tester  

### 🚀 Description  
GraphQL Authorization Tester is a tool that automates the testing of GraphQL mutations to determine whether they are allowed or restricted.  

✅ **Features:**  
- Discover all available mutations.  
- Identify allowed and forbidden mutations.  
- Works with any GraphQL endpoint.  
- Easy to install and use.  
- Easily integrates with Burp Suite for proxying and debugging, with no additional configuration required.
---

**🔁 Traffic Flow Diagram**
```
┌────────────────────┐
│  graphql-auth-tester│
│     (this tool)     │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│    Burp Suite       │  ← Intercepts & logs the requests
│  (Proxy: 127.0.0.1) │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│   SSH Tunnel (SOCKS)│  ← Created with: ssh -D 127.0.0.1:9001 ...
│  (127.0.0.1:9001)   │
└────────┬───────────┘
         │
         ▼
┌────────────────────┐
│   Target Web App    │
│  (GraphQL Endpoint) │
└────────────────────┘
```
The tool automatically extracts the target endpoint, headers, and body from the request file — allowing you to replay traffic exactly as Burp Suite captured it.

If Burp Suite is set up to listen on a proxy (e.g., 127.0.0.1:8080) and is configured to forward traffic through a SOCKS proxy (such as an SSH tunnel), your requests will seamlessly be visible in Burp without any extra flags.

## 📥 Installation  
You can install the tool directly using `go install`:  

```sh
go install github.com/ScarlyCodex/graphql-auth-tester@latest
```

Make sure `$HOME/go/bin/` is in your `$PATH`:
```sh
export PATH=$HOME/go/bin:$PATH
```

To do this permanently, you can add it to your `.bashrc` or `.zshrc`:
```sh
echo 'export PATH=$HOME/go/bin:$PATH' >> ~/.zshrc
source ~/.zshrc
```

## Usage
Once you have detected a request to a GraphQL endpoint, run `graphql-auth-tester --help`. 
- ⚠️ The `request.txt` must be in Burp Suite's format.
- Use `-r` to specify the path to your raw HTTP request file, e.g.:
  ```sh
  graphql-auth-tester -r request.txt
  ```
- Use `-t` to define the delay between each request in seconds (default is 1).
  ```sh
  graphql-auth-tester -r request.txt -t 2
  ```
This delay helps avoid rate-limiting or detection during testing by spreading out the requests.
Set it to 0 if you want the fastest possible execution (⚠️ not recommended on production targets).

If you want to perform unauthenticated checks, make sure to remove the neccesary headers e.g `Cookie:` || `Authorization:`. 

Finally:
```sh
graphql-auth-tester -r request.txt -t 7
```

### Note
As always, this tool could give false-positives since there could be tricky mutations which have to send very specific values. 
