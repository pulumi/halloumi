
import * as express from "express";
export class WebService {
    /**
     * @internal
     */
    __url: any;
    constructor(public name: string, public handler: (expressApp: express.Express) => void) {

    }
    discover(): string {
        return this.__url.get();
    }
}