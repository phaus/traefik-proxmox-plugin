# Traefik Plugin for Proxmox

This plugin analyses all configured VMs and CTs running in a specific Proxmox Cluster.  
Based on specific formattet content in the description (Notes) field, Traefik configurations can be exstracted.  
All lines in the format:

```ini
"traefik.…":"value"
```

Will be parsed as Traefik values.  

Example:  

```ini
"traefik.enable": "true" 
"traefik.http.routers.umbrelvm.entrypoints": "websecure" 
"traefik.http.routers.umbrelvm.rule": "Host(umbrelvm.org)" 
"traefik.http.routers.umbrelvm.tls": "true" 
"traefik.http.routers.umbrelvm.tls.certresolver": "leresolver" 
"traefik.http.services.umbrelvm.loadbalancer.server.port": "80"
```

This Content will be parsed as:  

```json
{
    "traefik.enable": "true",
    "traefik.http.routers.umbrelvm.entrypoints": "websecure",
    "traefik.http.routers.umbrelvm.rule": "Host(`umbrelvm.org`)",
    "traefik.http.routers.umbrelvm.tls": "true",
    "traefik.http.routers.umbrelvm.tls.certresolver": "leresolver",
    "traefik.http.services.umbrelvm.loadbalancer.server.port": "80"
}
```

## Required Permissions

For this plugin to be able to access all PVE API Endpoints, the following permissions need to be setup:

```txt
token:root@pam!traefik:0:0::
role:API-READER:Datastore.Audit,SDN.Audit,Sys.Audit,VM.Audit,VM.Config.Options:
acl:1:/:root@pam!traefik:API-READER:
```

## Config Values

You can configure the plugin using env variables.  
At the moment they are injected using [direnv](https://direnv.net/).  
For a detailed description have a look into the [.envrc.example](.envrc.example) file.  
