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
You can install the tool directly using `go install`:  

```sh
go install github.com/ScarlyCodex/gqlms@latest
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
Once you have detected a POST request to a GraphQL endpoint, run `gqlms --help`. 
- ⚠️ The `request.txt` must be in Burp Suite's or CAIDO's format.
- Use `-r` to specify the path to your raw HTTP request file, e.g.:
  ```sh
  gqlms -r request.txt
  ```
- Use `-t` to define the delay between each request in seconds (default is 1).
  ```sh
  gqlms -r request.txt -t 0
  ```
This delay helps avoid rate-limiting or detection during testing by spreading out the requests.
Set it to 0 if you want the fastest possible execution (⚠️ not recommended on production targets).

If you want to perform unauthenticated checks, make sure to remove the neccesary headers e.g `Cookie:` || `Authorization:`. 

Finally:
```sh
gqlms -r request.txt -t 5
```
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
