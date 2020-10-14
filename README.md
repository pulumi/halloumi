# halloumi

Pulumi + Heroku = Halloumi

You write your application, we run it in the cloud.

`halloumi` is a prototype and not under active development. 

## Summary

The halloumi SDK defines an application model where you simply write your http handler, and we take care of the rest:

```go
app := app.NewApp("petStore")

// a cloud web service that returns a number [0-100)
randomNumberService := web.NewWebService("randomNumber", func() http.Handler {
    // a normal HTTP handler, halloumi takes care of running this in the cloud
    r := mux.NewRouter()
    handler := func(w http.ResponseWriter, r *http.Request) {
        rand.Seed(time.Now().UnixNano())
        fmt.Fprintf(w, "%d", rand.Intn(100))
    }
    r.HandleFunc("/", handler)
    return r
})

app.Register(randomNumberService)
```

To deploy this app, simply run your program with the `halloumi` tool:

```shell
$ halloumi main.go
Deployming app petStore
Deploying service randomNumber
Successfully deployed service randomNumber
Service randomNumber running at: web-lb-f1f8017-934112424.us-west-2.elb.amazonaws.com
Successfully deployed app petStore
```
### Cross-service Dependencies
Halloumi allows consuming services from other services:

```go

// a cloud web service that returns N of a random animal.
// this service takes a dependency on the URLs of the previously defined services
nAnimalsService := web.NewWebService("nAnimals", func() http.Handler {
    r := mux.NewRouter()
    var num string
    var animal string
    handler := func(w http.ResponseWriter, r *http.Request) {
        // Notice how we have the URL of randomNumberService
        // available to consume here!
        numResp, err := http.Get(randomNumberService.URL())
        if err != nil {
            fmt.Fprintf(w, err.Error())
        }
        defer numResp.Body.Close()

        if numResp.StatusCode == http.StatusOK {
            bodyBytes, err := ioutil.ReadAll(numResp.Body)
            if err != nil {
                log.Fatal(err)
            }
            num = string(bodyBytes)
        }

        // Notice how we have the URL of randomAnimalService
        // available to consume here!
        animalResp, err := http.Get(randomAnimalService.URL())
        if err != nil {
            fmt.Fprintf(w, err.Error())
        }
        defer numResp.Body.Close()

        if animalResp.StatusCode == http.StatusOK {
            bodyBytes, err := ioutil.ReadAll(animalResp.Body)
            if err != nil {
                log.Fatal(err)
            }
            animal = string(bodyBytes)
        }

        fmt.Fprintf(w, "Wow, you got %s %ss!", num, animal)
    }
    r.HandleFunc("/", handler)
    return r
})
```

## installation
1. Clone repo to $GOPATH
2. make (to install the halloumi tool)
3. run the included example `halloumi main.go`

## prereqs

1. authenticated pulumi CLI (>= v2.10.1)
2. AWS CLI + creds
3. Docker

## How does it work?
The halloumi orchestrator works off of the principal of invoking the program different sets of environment variables

1. First pass `DRY_RUN`: this procudes a list of apps and services to deploy allowing the program to exit without starting any http servers.
2. The orchestrator deploys each app and service, building a docker image from the provided `main.go` file.
3. When a service is deployed, an environment variable `HALLOUMI_APPNAME_SVCNAME` is set in the ECS task. When the program is run with that environment variable, the program will start a web server for the specified service. 

## TODOs:
1. Can we get rid of the CLI? This demo would be more powerful without it.
2. Auto-generate the docker file side by side. Right now it is just checked in along side the example.
3. Add some other components besides web services (blob, queue, db).
4. Write an SDK for a language like javascript. The SDK layer is super thin and wouldn't be much work. 
5. Perf/parallelism. Right now to be lazy, I don't detect dependencies and take advantage of the fact that declaration order is also dependency order. This just means that deployments are serial and slow. 