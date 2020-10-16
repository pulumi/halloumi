
import * as express from "express";
export class WebService {
    /**
     * @internal
     */
    __url: any;
    __local: boolean;
    constructor(public name: string, public handler: (expressApp: express.Express) => void) {

    }
    discover(): string {
        if (this.__local) {
            return this.__url
        }
        return this.__url.get();
    }
}