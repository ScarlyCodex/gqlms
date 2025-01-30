# 🔥 GraphQL Authorization Tester  

### 🚀 Description  
GraphQL Authorization Tester is a tool that automates the testing of GraphQL mutations to determine whether they are allowed or restricted.  

✅ **Features:**  
- Discover all available mutations.  
- Identify allowed and forbidden mutations.  
- Works with any GraphQL endpoint.  
- Easy to install and use.  

---

## 📥 Installation  
You can install the tool directly using `go install`:  

```sh
go install github.com/ScarlyCodex/graphql-auth-tester@latest
```

## Usage
Once you have detected a request to a GraphQL endpoint, you could copy-paste it to the request.txt (Burp Suite's format), the `-r` is to determine the `.txt` file of your request and the `=t` one is for the time in seconds between each authorization check request. 

If you want to perform unauthenticated checks, make sure to remove the neccesary headers e.g `Cookie:` || `Authorization:`. 

Finally:
```sh
graphql-auth-tester -r request.txt -t 7
```

### Note
As always, this tool could give false-positives since there could be tricky mutations which have to send very specific values. 
