import { WebService } from "../WebService";
import * as awsx from "@pulumi/awsx";
import * as aws from "@pulumi/aws";
import * as serverlessExpress from "aws-serverless-express";

import * as express from "express";
import { InlineProgramArgs, LocalWorkspace } from "@pulumi/pulumi/x/automation";

export class App {
    services: WebService[];
    constructor() {
        this.services = [];
    }
    addService(svc: WebService) {
        this.services.push(svc);
    }

    async run() {

        const args = process.argv.slice(2);
        let local = false;
        if (args.length > 0 && args[0]) {
            local = args[0] === "local";
        }

        if (local) {
            await this.deployLocal();
        }
        else {
            await this.deployAWS()
        }
        
    }

    async deployAWS() {
        const pulumiProgram = async () => {
            const outputs: { [key: string]: any; } = {};

            // for each service create a lambda + api gateway
            for (let svc of this.services) {

                // TODO we're going to use API gateway in a similar fashion to the
                // cloud lib using a lambda callback factory
                // if this doesn't work unexpectedly, we may need to use an explicit provider
                const name = svc.name;

                const createRequestListener = () => {
                    const app = express();
                    svc.handler(app)
                    return app
                };



                const entryPointFactory: aws.lambda.CallbackFactory<awsx.apigateway.Request, awsx.apigateway.Response> = () => {
                    // Pass */* as the binary mime types.  This tells aws-serverless-express to effectively
                    // treat all messages as binary and not reinterpret them.
                    const server = serverlessExpress.createServer(
                        createRequestListener(), /*serverListenCallback*/ undefined, /*binaryMimeTypes*/["*/*"]);

                    // All the entrypoint function for the Lambda has to do is pass the events along to the
                    // server we created above.  That server will then forward the messages along to the
                    // request listener provided by the caller.
                    return (event, context) => {
                        serverlessExpress.proxy(server, event, <any>context);
                    };
                };

                const cb = new aws.lambda.CallbackFunction("cb_" + name, {
                    policies: [aws.iam.ManagedPolicy.AWSLambdaFullAccess],
                    callbackFactory: entryPointFactory,
                })

                const api = new awsx.apigateway.API("gateway_" + name, {
                    // Register two paths in the Swagger spec, for the root and for a catch all under the
                    // root.  Both paths will map to the single AWS lambda created above.
                    routes: [
                        {
                            path: "/",
                            method: "ANY",
                            eventHandler: cb,
                        },
                        {
                            path: "/{proxy+}",
                            method: "ANY",
                            eventHandler: cb,
                        },
                    ],
                });

                // add .get function to service registry so that future services can depend on it at runtime
                svc.__url = api.url;

                // add url to exports
                outputs[name] = api.url
            }
            return outputs;
        }

        const args: InlineProgramArgs = {
            stackName: "dev1",
            projectName: "halloumijs",
            program: pulumiProgram,
        };

        const stack = await LocalWorkspace.createOrSelectStack(args, { workDir: process.cwd() });
        await stack.workspace.installPlugin("aws", "v3.6.1");
        await stack.setConfig("aws:region", { value: "us-west-2" });
        const upRes = await stack.up({ onOutput: console.info });
        for (let svc of this.services) {
            console.log(`service "${svc.name}" running at: ${upRes.outputs[svc.name].value}`);
        }
    }

    async deployLocal() {
        for (let svc of this.services) {
            const app = express();
            svc.handler(app)
            const server = app.listen();
            svc.__local = true;
            svc.__url = `http://localhost:${(server.address() as any).port}`;
            console.log(`service "${svc.name}" running at: ${svc.__url}`);
        }
    }
}