
# Gravity Log Application

Gravity Log Application is intended for gathering logs and providing API for logs retrieval in Gravity cluster. The application is based on [Logrange](https://github.com/logrange/logrange) streaming database.

### 1. HTTP API

Listens *:8083

#### Query logs

###### GET: /v1/log?limit=&query=

- `query` param terms: "pod:<pod_name>", "file:<container_id>", "container:<container_name>", "and" or "or"
  
  **Example**:<br/>
  `/v1/log?limit=1&query=pod:p1 and container:"c1" and file:f1 or file:f2`
  
- `query` param could be used to search for literal text occurrence:
  
  **Example**:<br/>
  `/v1/log?limit=100&query="some text"`

- default `limit` is 1000

- output format: JSON, e.g.: `[{"type":"data","payload":""}]`

#### Download (the latest) logs

###### GET: /v1/download

- output format: TEXT files, compressed as `tar.gz` archive

### 2. Recurring jobs
#### Config synchronizer

Job that syncs forwarders configurations (Gravity "log-forwarders" k8s configMap to Logrange "lr-forwarder" k8s configMap), so that as soon as Gravity forwarders config is updated Logrange have it's configuration updated as well (within couple minutes).

#### Queries executor

Job that runs preconfigured queries by schedule. In particular it is used to keep logs database disk usage within limits and perform truncation when needed.

### 3. Build

**Run commands in order:**<br/>

 1. Clone repo:<br/>
 `git clone git@github.com:gravitational/logging-app.git`
 2. Enter logging-app directory:<br/>
 `cd logging-app`
 3. Build Gravity package (specify `OPS_URL` and `VERSION` as needed):<br/>
 `OPS_URL= VERSION=x.x.x make clean tarball`

