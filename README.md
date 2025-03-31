# 🔥 GraphQL Authorization Tester  

## 🚀 Description  
GraphQL Authorization Tester is a tool that automates the testing of GraphQL mutations to determine whether they are allowed or restricted.  

✅ **Features:**  
- Discover all available mutations.  
- Identify allowed and forbidden mutations.  
- Works with any GraphQL endpoint.  
- Easy to install and use.  
- Easily integrates with Burp Suite, CAIDO, or any toolset you use for proxying and debugging, with no additional configuration required.

## 📥 Installation  

Make sure `$HOME/go/bin/` is in your `$PATH`:
```sh
export PATH=$HOME/go/bin:$PATH
```

To do this permanently, you can add it to your `.bashrc` or `.zshrc`:
```sh
echo 'export PATH=$HOME/go/bin:$PATH' >> ~/.zshrc
source ~/.zshrc
```
Finally — you install the tool directly using `go install`:
```sh
go install github.com/ScarlyCodex/gqlms@latest
```
## Usage
Once you have detected a POST request to a GraphQL endpoint, run `gqlms --help`. 
- ⚠️ The `request.http` must be in raw HTTP format as exported from tools like Burp Suite, CAIDO, or similar.

#### Required Options
- Use `-r` to specify the path to your raw HTTP request file, e.g.:
  ```sh
  gqlms -r request.http
  ```
- Use `-t` to define the delay between each request in seconds (default is 1).
  ```sh
  gqlms -r request.http -t 0
  ```
This delay helps avoid rate-limiting or detection by spreading out the requests.
Set it to 0 for fastest execution — ⚠️ Not recommended on production targets

#### 🔐 Auth & Unauthenticated Testing
This is useful for testing privilege escalation scenarios or misconfigured access controls.

If you want to test how the GraphQL server behaves without credentials (unauthenticated), but introspection requires authentication, you can use the new -unauth flag:
```sh
gqlms -r request.http -unauth=Authorization,Cookie
```
- The tool will use the full request (with headers) to fetch the schema
- Then it will remove the specified headers and test each mutation unauthenticated

⚠️ Important notes:
- The list is case-sensitive (e.g., Authorization ≠ authorization)
- Each header name must be comma-separated with no spaces between:
   ```
    -unauth=Authorization,Cookie,X-Custom-Token
   ```
If any of the specified headers are not present in the request, you'll be prompted whether you wish to continue or not.

#### 🔌 Proxy Modes
You can optionally route all requests through a proxy (e.g. Burp Suite or another proxy server):

**🔹 No Proxy (default)**
```sh
gqlms -r request.http -t 1
```
**🔹 Use default proxy (`http://127.0.0.1:8080`)**
```sh
gqlms -r request.http -t 2 -proxy=
```
**🔹 Use a custom proxy**
```sh
gqlms -r request.http -t 3 -proxy=http://192.168.1.100:8888
```
#### 🌐 HTTPS Support (`-ssl` flag)

By default, the tool assumes GraphQL endpoints use HTTPS.

- If the URL in your `request.http` does not specify a protocol (`http://` or `https://`), the tool will **automatically assume `https://`**.

- To override this behavior and force HTTP (e.g., in a local lab), use:
```sh
gqlms -r request.http -ssl=false
```


##### 📄 Output Files
After the tool completes its testing phase—three `.txt` files will be created in your current working directory:

- `allMutations.txt`  
  Contains all mutation names discovered via GraphQL introspection.

- `allowedMutations.txt`  
  Lists the mutations that were accessible (i.e., not blocked by authorization logic).

- `unallowedMutations.txt`  
  Lists the mutations that were **restricted** (i.e., returned authorization errors or access was denied).

You can review these files to quickly understand which parts of the GraphQL API are exposed to the current user context.


## 🔐 Authorization Logic in GraphQL Endpoints

### Is this Authorization Logic Always Applicable in GraphQL Endpoints?
##### Yes – Generally True Across All GraphQL Implementations

While the exact mechanics may vary depending on the implementation (Apollo, Graphene, Hasura, Sangria, etc.), the underlying authorization model follows a consistent architecture.

---
**🧱 Standard Execution Layers in GraphQL**

GraphQL operates in three key layers:

1. Introspection

   - Used to discover the schema (__schema, __type, etc.)

   - May or may not be accessible depending on backend configuration

2. Validation

   - Ensures the query matches the schema types and argument definitions

   - Happens before any resolver is called

  3. Execution (Resolvers)

     - Where the actual logic and authorization checks happen

     - This is where "not authorized" errors are generated

---
**📌 Decision Flow — Understanding GraphQL Server Responses**
```
📌 Interpreting GraphQL Mutation Responses

1️⃣ Validation Errors (400 BAD REQUEST)
───────────────────────────────────────
Example:
{
  "errors": [
    { "message": "Unknown argument 'input' on field ..." },
    { "message": "Field 'host' of type 'String!' is required ..." }
  ]
}
✔️ This means:
- The mutation exists and is reachable
- The current user is allowed to invoke it
- The error is due to incorrect format, not authorization

🛠️ Action: Fix argument structure or types in the mutation

2️⃣ Authorization Errors (403 / 401 or GraphQL error)
───────────────────────────────────────
Example:
{
  "errors": [{
    "message": "Not authorized",
    "extensions": { "code": "UNAUTHENTICATED" }
  }]
}
Or HTTP-level errors:
- 401 Unauthorized
- 403 Forbidden

❌ This means:
- The mutation exists, but the current user is **not authorized**
- The resolver explicitly blocks execution

🛠️ Action: Authenticate or escalate privileges

3️⃣ Mutation Hidden in Introspection
───────────────────────────────────────
❌ The mutation is **not visible** in the schema at all

🔐 Indicates:
- Schema-level authorization
- Backend filters the schema based on user roles

🛠️ Action: Look for roles or users with broader access
```
---
**📚 Recommended References**

| Resource                                                                                     | Description                                                        |
|----------------------------------------------------------------------------------------------|--------------------------------------------------------------------|
| [GraphQL Specification](https://spec.graphql.org)                                            | Official spec covering execution, validation, types                |
| [Apollo Server Security Docs](https://www.apollographql.com/docs/apollo-server/security/authentication/) | Explains resolver-based authentication and schema control          |
| [OWASP GraphQL Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/GraphQL_Security_Cheat_Sheet.html) | Great practical advice for securing GraphQL                        |
| [Graphene Python Docs – Auth](https://docs.graphene-python.org/en/latest/execution/authentication/) | Shows how auth is applied in resolvers (Python context)            |


#### Note
As always, this tool could give false-positives since there could be tricky mutations which have to send very specific values. 
