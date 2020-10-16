# Halloumi in Node.js

This version of halloumi creates a library that can be used with independently executable programs. You still get multi-service dependency, but you don't need a higher-level CLI. Just `yarn start` and you're off to the races.

This library also supports a super speedy local development mode (if a 20 second lambda deployment is too slow of a dev loop). Just run `yarn local` and your services will run on `localhost`.

```typescript
const application = new App();

const svc1 = new WebService("hello", (expressApp: express.Express) => {
    expressApp.get('/', (req, res) => res.send("omg i'm alive\n"));
});
application.addService(svc1);


const svc2 = new WebService("world", (expressApp: express.Express) => {
    expressApp.get('/', async (req, res) => {
        const svc1Url = svc1.discover();
        const result = await (await fetch(svc1Url)).text()
        
        res.send(`this is the world. With a window from hello:\n${result} \n`);
    });
});
application.addService(svc2);

application.run().catch(err => { console.error(err) });
```

## Running

`yarn start`:

```shell
$ yarn start
yarn run v1.22.10
$ tsc && AWS_REGION=us-west-2 node ./bin/index.js
Updating (dev1)


View Live: https://app.pulumi.com/EvanBoyle/halloumijs/dev1/updates/8




    pulumi:pulumi:Stack halloumijs-dev1 running 

    aws:apigateway:x:API gateway_hello  
    aws:apigateway:x:API gateway_world  

    aws:iam:Role cb_hello  
    aws:iam:Role cb_world  

    aws:iam:RolePolicyAttachment cb_hello-32be53a2  
    aws:iam:RolePolicyAttachment cb_world-32be53a2  

    aws:lambda:Function cb_hello  

    aws:apigateway:RestApi gateway_hello  

    aws:apigateway:Deployment gateway_hello  

    aws:lambda:Permission gateway_hello-07686014  
    aws:lambda:Permission gateway_hello-e5c3be79  

    aws:apigateway:Stage gateway_hello  

 ~  aws:lambda:Function cb_world updating [diff: ~code]

 ~  aws:lambda:Function cb_world updated [diff: ~code]

    aws:apigateway:RestApi gateway_world  

    aws:apigateway:Deployment gateway_world  

    aws:lambda:Permission gateway_world-e5c3be79  
    aws:lambda:Permission gateway_world-07686014  

    aws:apigateway:Stage gateway_world  

    pulumi:pulumi:Stack halloumijs-dev1  
 

Outputs:
    hello: "https://f5cvoceykd.execute-api.us-west-2.amazonaws.com/stage/"
    world: "https://nk9u34tf0g.execute-api.us-west-2.amazonaws.com/stage/"


Resources:
    ~ 1 updated
    18 unchanged

Duration: 4s


service "hello" running at: https://f5cvoceykd.execute-api.us-west-2.amazonaws.com/stage/
service "world" running at: https://nk9u34tf0g.execute-api.us-west-2.amazonaws.com/stage/
✨  Done in 17.90s.

$ curl https://f5cvoceykd.execute-api.us-west-2.amazonaws.com/stage/
omg i\'m alive

$ curl https://nk9u34tf0g.execute-api.us-west-2.amazonaws.com/stage/
this is the world. With a windo from hello:
omg i\'m alive
 
!!!!!!!wow!!!!!!!
```

### Local Mode

Just run `yarn local` to get the services running on `localhost`:

```shell
$ yarn local
yarn run v1.22.10
$ tsc && AWS_REGION=us-west-2 node ./bin/index.js local
service "hello" running at: http://localhost:60957
service "world" running at: http://localhost:60958

$ curl http://localhost:60957
omg i\'m alive

$ curl http://localhost:60958
omg i\'m alive
 
!!!!!!!wow!!!!!!!
```