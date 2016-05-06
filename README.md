
### Deploy

Deploy utilizes modified libcompose to apply network policies on an application composition. 

#### How to try it out

###### 1. Get netplugin vagrant setup:
Bring up Contiv Vagrant setup as in [Step 1. Contiv Netplugin](https://github.com/contiv/netplugin/README.md).
```
$ cd $HOME; mkdir -p deploy/src/github.com/contiv
$ export GOPATH=$HOME/deploy
$ cd deploy/src/github.com/contiv/
$ git clone https://github.com/contiv/netplugin
$ cd netplugin; make demo
```

###### 2. Get libcompose and enter VM
```
$ mkdir -p $GOPATH/src/github.com/docker
$ cd $GOPATH/src/github.com/docker
$ git clone https://github.com/jainvipin/libcompose
$ cd $GOPATH/src/github.com/contiv/netplugin
$ make ssh
```

###### 3. Inside the VM, compile libcompose
```
$ cd $GOPATH/src/github.com/docker/libcompose
$ git checkout deploy
$ make binary
$ ln -s $GOPATH/src/github.com/docker/libcompose/bundles/libcompose-cli /opt/gopath/bin/contiv-compose
```

###### 4. Build or Get container images

You can either build your own images or download prebuilt sample images needed. For if you choose to use the
standard docker images, you can:
```
$ cd $GOPATH/src/github.com/docker/libcompose/deploy/example/app
$ docker build -t web .
```
or 
```
$ docker pull jainvipin/web
```

Similarly build the db image
```
$ cd $GOPATH/src/github.com/docker/libcompose/deploy/example/db
$ docker build -t redis -f Dockerfile.redis .
```
or 
```
$ docker pull jainvipin/redis
```

###### 5. Run contiv-compose and see the policies working

Before we run the composition, we must create a few networks, which can be easily accomplished using:
```
netctl net create -s 10.11.1.0/24 dev
```

Now we can fire up the example composition
```
$ cd $GOPATH/src/github.com/docker/libcompose/deploy/example
$ contiv-compose up -d
```

You'll see something like, which is an indication that the composition is up and running
```
WARN[0000] Note: This is an experimental alternate implementation of the Compose CLI (https://github.com/docker/compose) 
INFO[0000] Creating policy contract from 'web' -> 'redis' 
INFO[0000] Using default policy 'TrustApp'...           
INFO[0000] User 'vagrant': applying 'TrustApp' to service 'redis' 
INFO[0000]   Fetched port/protocol) = tcp/5001 from image 
INFO[0000]   Fetched port/protocol) = tcp/6379 from image 
INFO[0000] Project [example]: Starting project          
INFO[0000] [0/2] [web]: Starting                        
INFO[0000] [0/2] [redis]: Starting                      
INFO[0000] [1/2] [redis]: Started                       
INFO[0001] [2/2] [web]: Started        
```

What just happened was that for user `vagrant` contiv-compose picked up the default policy of `TrustApp`
This policy can be found in `ops.json` file, which is a modifiable ops policy in the same directory. 
As per ops.json TrustApp policy permits all ports allowed by the application therefore in the above 
run we observed that contiv-compose attempts to fetch the port information from the redis image and 
applies inbound set of rules to it

Now, let's try to verify whether the isolation policy is working as expected
```
$ docker exec -it example_web_1 /bin/bash
< ** inside container ** >
$ nc -zvw 1 example_redis 6375-6380
example_redis.dev.default [10.11.1.21] 6380 (?) : Connection timed out
example_redis.dev.default [10.11.1.21] 6379 (?) open
example_redis.dev.default [10.11.1.21] 6378 (?) : Connection timed out
example_redis.dev.default [10.11.1.21] 6377 (?) : Connection timed out
example_redis.dev.default [10.11.1.21] 6376 (?) : Connection timed out
example_redis.dev.default [10.11.1.21] 6375 (?) : Connection timed out

$ exit
< ** back to linux prompt ** >
```

###### 6. Tear down the composition
Let us tear down the composition and associated policies we started earlier
```
$ cd $GOPATH/src/github.com/docker/libcompose/deploy/example
$ contiv-compose stop
```

#### Playing with a few more interesting cases

###### 1. Trying to scale an application tier
One can scale any application tier and expect that the policy be applied to the group of containers belonging to
a tier/service/group. To follow up with previous example, if we try to scale web tier as follows:
```
$ contiv-compose up -d
$ contiv-compose scale web=5
```
With this now we can go into any of the web tier container and experiment our policy verification. For example:
```
$ docker exec -it example_web_3 /bin/bash
< ** inside container ** >
$ nc -zvw 1 example_redis 6375-6380
example_redis.dev.default [10.11.1.21] 6380 (?) : Connection timed out
example_redis.dev.default [10.11.1.21] 6379 (?) open
example_redis.dev.default [10.11.1.21] 6378 (?) : Connection timed out
example_redis.dev.default [10.11.1.21] 6377 (?) : Connection timed out
example_redis.dev.default [10.11.1.21] 6376 (?) : Connection timed out
example_redis.dev.default [10.11.1.21] 6375 (?) : Connection timed out

$ exit

$ contiv-compose stop
$ contiv-compose rm -f
```

###### 2. Changing the default network which 
Create a new `test` network first
```
netctl net create -s 10.22.1.0/24 test
```

Then start a composition in the new network. For this we'll edit the docker-compose.yml to look like following:
```
web:
  image: web
  ports:
   - "5000:5000"
  links:
   - redis
  net: test
redis:
  image: redis
  net: test
```

And fire up the composition using 
```
$ contiv-compose up -d
```

The new composition is started in the `test` network as specified in the composition. We can of course veify the policy, etc. between the containers now and bring the composition down using

```
$ contiv-compose stop
```

###### 3. Specfying an override policy

Should there be a need to specify an override policy for a service tier, we can use a policy label to do so as 
in the following modified yml
```
web:
  image: web
  ports:        
   - "5000:5000"
  links:
   - redis
  net: test
redis:
  image: redis  
  net: test       
  labels:         
   io.contiv.policy: "RedisDefault" 
```

You would note that override policies for various users is specified outside the application composition, and
in operational policy file (ops.json), which states that vagrant user is allowed to use following policies:
```
                { "User":"vagrant", 
                  "DefaultTenant": "default",
                  "Networks": "test,dev",
                  "DefaultNetwork": "dev",
                  "NetworkPolicies" : "TrustApp,RedisDefault,WebDefault",
                  "DefaultNetworkPolicy": "TrustApp" }
```

More over the override policy called `RedisDefault` is later defined as 
```
                { "Name":"RedisDefault", 
                  "Rules": ["permit tcp/6379", "permit tcp/6378", "permit tcp/6377"] },
```

So, at this point we can go ahead and fire up the composition and verify that appropriate ports are open
```
$ contiv-compose up -d

$ docker exec -it example_web_1 /bin/bash
< ** inside container ** >
$ nc -zvw 1 example_redis 6375-6380
example_redis.test.default [10.22.1.26] 6380 (?) : Connection timed out
example_redis.test.default [10.22.1.26] 6379 (?) open
example_redis.test.default [10.22.1.26] 6376 (?) : Connection timed out
example_redis.test.default [10.22.1.26] 6375 (?) : Connection timed out
```

Note that in above output, ports 6377-6379 are not `Connection timed out`, which means that network is 
not dropping the packet towards target example_redis service

Let's cleanup/stop the composition, before moving to other things

```
$ contiv-compose stop
```

###### 4. Verifying that only allowed networks are permitted

For a composition that attempts to speicfy a network that is not permitted for it, contiv-compose will error out.
For this we'll create a production network.

```
$ netctl net create -s 10.33.1.0/24 production

$ cat docker-compose.yml
web:
  image: web
  ports:
   - "5000:5000"
  links:
   - redis
  net: production
redis:
  image: redis
  net: production

$ contiv-compose up -d
WARN[0000] Note: This is an experimental alternate implementation of the Compose CLI (https://github.com/docker/compose) 
ERRO[0000] User 'vagrant' not allowed on network 'production' 
```

###### 5. Verifying that only allowed policies are permitted
For a composition that attempts to speicfy a disallowed policy, contiv-compose will error out.
For this we'll specify `AllPriviliges` policy for the user `vagrant, and as expected we get errored out as follows:

```
$ cat docker-compose.yml
web:
  image: web
  ports:
   - "5000:5000"
  links:
   - redis
redis:
  image: redis
  labels:
   io.contiv.policy: 'AllPriviliges"

$ contiv-compose up -d
WARN[0000] Note: This is an experimental alternate implementation of the Compose CLI (https://github.com/docker/compose) 
INFO[0000] Creating policy contract from 'web' -> 'redis' 
ERRO[0000] User 'vagrant' not allowed to use policy 'AllPriviliges' 
ERRO[0000] Error obtaining policy : Deny disallowed policy  
ERRO[0000] Failed to apply in-policy for service 'redis': Deny disallowed policy 
FATA[0000] Failed to Create Network Config: Deny disallowed policy 
```

###### 6. Specifying a override tenant (non default)

We can use contiv-compose yml to specify a non default tenant to run the applications in a different tenant.
While users are typically not allowed to specify the tenancy, rather it is picked up from the user's context,
this example is kept here just for illustration:

For this let's create a new tenant `blue` and specify a network `dev` in `blue` tenant

```
netctl tenant create blue
netctl net create -t blue -s 10.11.2.0/24 dev
```

Now if we create a composition that states the tenancy, we can use it as follows:

```
$ cat docker-compose.yml
web:
  image: web
  ports:
   - "5000:5000"
  links:
   - redis
  labels:
   io.contiv.tenant: "blue"
redis:
  image: redis
  labels:
   io.contiv.tenant: "blue"

$ contiv-compose up -d

$ docker inspect example_web_1 | grep \"IPAddress\"
        "IPAddress": "",
                "IPAddress": "10.11.2.23",

```

Note that it allocated an IP from blue tenant's IP pool


#### Some Notes and Comments
- This tool is used to demonstration the automation and integration with Contiv Networking and is not meant to
be used in production.
- The user based authentication is expected to keep the operational policy `ops.json` as docker's authorization 
plugin to permit only authenticated users to specify certain operations
- Hacking on contiv's libcompose version is welcome! Please make sure you run unit/sanity tests before 
submitting a PR. It could be easily done by `make test-deploy` and 'make test-unit` - if both succeed 
you are good
