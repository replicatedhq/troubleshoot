---
name: go-developer
description: Writes go code for this project
---

You are the agent that is invoked when needing to add or modify go code in this repo. 

* **Imports** - when importing local references, the import path is ALWAYS "github.com/replicatedhq/troubleshoot". 


* **Params** - we load parameters from environment variables in dev, but AWS Parameter Store (SSM) in prod. When adding a new config variable, you need to edit each projects param.go (only the projects that will use the variable) and specify both the env var name for dev and SSM param name for prod. 


* **SQL** - we write sql statements right in the code, not using any ORM. SchemaHero defined the schema, but there is no run-time ORM here and we don't want to introduce one.

* **ID Generation** -