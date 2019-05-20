
# Gravity Log Application

Gravity Log Application is intended for gathering logs and providing API for logs retrieval in Gravity cluster. The application is based on [Logrange](https://github.com/logrange/logrange) streaming database.

### 1. HTTP API

The server listens on port 8083 by default.

#### Querying Logs

###### GET: /v1/log?limit=&query=

- `query` can contain the following terms:<br/>
   * `pod`:<name> - to limit search to a specific pod<br/>
   * `container`:<name> - to limit search to a specific container inside a pod<br/>
   * `file`:<file> - to limit search to a specific log file<br/>
   * boolean operators `and` and `or`

  **Example**:<br/>
  `/v1/log?limit=1&query=pod:p1 and container:"c1" and file:f1 or file:f2`
  
- `query` param could be used to search for literal text occurrence:
  
  **Example**:<br/>
  `/v1/log?limit=100&query="some text"`

- default `limit` is 1000

- output format: JSON, e.g.: `[{"type":"data","payload":""}]`

#### Download logs

###### GET: /v1/download

- output format: compressed tarball stream (`tar.gz`)

### 2. Recurring jobs

The application has a set of recurring jobs which run as a part of the application binary and perform tasks described below.

#### Config synchronizer

Job that syncs the updates to Gravity log forwarder configuration to internal format. Frequency can be configured (see `SyncIntervalSec` param of the config) and is set to 20 seconds by default.

#### Queries executor

Job that runs scheduled queries. Queries can be configured (see `CronQueries` section of the config), by default there is a single query configured which is used to keep the database size within limits by periodically trimming older entries.

### 3. Build instructions

```
 $ git clone git@github.com:gravitational/logging-app.git
 $ cd logging-app
 $ make clean tarball
 ```
 Prefix the `make` command with `OPS_URL=ops-url` to work with a remote package repository and `VERSION=x.y.z` to build a specific version.
