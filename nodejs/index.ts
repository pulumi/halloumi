import * as express from 'express';
import { WebService } from "./webService";
import { App } from "./app";
import fetch from "node-fetch";

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