
Piping sequence:
 $ tail -F <filePath> | grep -E <expr> --line-buffered | awk '{$1=$2=$3="";print $0} 
 $ tail -F <filePath> | grep -E <expr> --line-buffered | cut -d' ' -f4- 

The naming schema used by kubelet to create symlinks to container log files:

SymlinkName = "{pod.Name}}_{{pod.Namespace}}_{{container.Name}}-{{dockerId}}.log"
SymlinkPath = "/var/log/containers/{{SymlinkName}}"

Matchers:

Timestamp: ^\d{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.\d+Z
grep -E  : ^[[:digit:]]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[[:digit:]]+Z
Pod name: [a-zA-Z\0-9-]+

SymlinkName = "{{pod.Name}}_{{pod.Namespace}}_{{container.Name}}-{{dockerId}}.log"

grep -E '^[[:digit:]]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[[:digit:]]+Z\s+[a-zA-Z\0-9-]+'

with placeholders for pod name
pod name: [^\_]+
by container `kubedns`:
grep -E '^[[:digit:]]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[[:digit:]]+Z[[:space:]]+[a-zA-Z\0-9-]+[^\_]+_[^\_]+_kubedns'
multiple containers:
grep -E '^[[:digit:]]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[[:digit:]]+Z[[:space:]]+[a-zA-Z\0-9-]+[^\_]+_[^\_]+_kubedns'
