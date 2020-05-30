# kube-controller-manager 源码分析(基于release-1.18 branch)

**NOTE: 由于代码篇幅太多，在分析的过程中会将不重要的部分删除，我将用//..................代替了.**

## 函数入口在[controller-manager.go](https://github.com/kubernetes/kubernetes/blob/release-1.18/cmd/kube-controller-manager/controller-manager.go)

``` golang
func main() {
    rand.Seed(time.Now().UnixNano())

    command := app.NewControllerManagerCommand()

    // TODO: once we switch everything over to Cobra commands, we can go back to calling
    // utilflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
    // normalize func and add the go flag set by hand.
    // utilflag.InitFlags()
    logs.InitLogs()
    defer logs.FlushLogs()

    if err := command.Execute(); err != nil {
        os.Exit(1)
    }
}
```

## 进入[NewControllerManagerCommand](https://github.com/kubernetes/kubernetes/blob/release-1.18/cmd/kube-controller-manager/app/controllermanager.go#L92:6)看下实现细节

看到用的是[cobra](https://github.com/spf13/cobra)的库(一个非常牛的CLI的库，在很多项目中都有使用，这里不讲cobra具体怎么使用)。这里定义了一个函数赋值给了cobra.Commnad的Run成员函数接口，它将在main函数中调用command.Execute()后被执行。

```go
// NewControllerManagerCommand creates a *cobra.Command object with default parameters
func NewControllerManagerCommand() *cobra.Command {
   //..................

    cmd := &cobra.Command{
        Use: "kube-controller-manager",
        Long: `The Kubernetes controller manager is a daemon that embeds
the core control loops shipped with Kubernetes. In applications of robotics and
automation, a control loop is a non-terminating loop that regulates the state of
the system. In Kubernetes, a controller is a control loop that watches the shared
state of the cluster through the apiserver and makes changes attempting to move the
current state towards the desired state. Examples of controllers that ship with
Kubernetes today are the replication controller, endpoints controller, namespace
controller, and serviceaccounts controller.`,
        Run: func(cmd *cobra.Command, args []string) {
            verflag.PrintAndExitIfRequested()
            utilflag.PrintFlags(cmd.Flags())

            c, err := s.Config(KnownControllers(), ControllersDisabledByDefault.List())
            if err != nil {
                fmt.Fprintf(os.Stderr, "%v\n", err)
                os.Exit(1)
            }

            if err := Run(c.Complete(), wait.NeverStop); err != nil {
                fmt.Fprintf(os.Stderr, "%v\n", err)
                os.Exit(1)
            }
        },
    }

    //..................

    return cmd
}
```

然后这个函数中调到了真正的Run函数，这个函数里面才是进行真正的启动kube-controller-manager的逻辑处理.

```go
// Run runs the KubeControllerManagerOptions.  This should never exit.
func Run(c *config.CompletedConfig, stopCh <-chan struct{}) error {
    // To help debugging, immediately log version
    klog.Infof("Version: %+v", version.Get())

    if cfgz, err := configz.New(ConfigzName); err == nil {
        cfgz.Set(c.ComponentConfig)
    } else {
        klog.Errorf("unable to register configz: %v", err)
    }

    // Setup any healthz checks we will want to use.
    var checks []healthz.HealthChecker
    var electionChecker *leaderelection.HealthzAdaptor
    if c.ComponentConfig.Generic.LeaderElection.LeaderElect {
        electionChecker = leaderelection.NewLeaderHealthzAdaptor(time.Second * 20)
        checks = append(checks, electionChecker)
    }

    // Start the controller manager HTTP server
    // unsecuredMux is the handler for these controller *after* authn/authz filters have been applied
    var unsecuredMux *mux.PathRecorderMux
    if c.SecureServing != nil {
        unsecuredMux = genericcontrollermanager.NewBaseHandler(&c.ComponentConfig.Generic.Debugging, checks...)
        handler := genericcontrollermanager.BuildHandlerChain(unsecuredMux, &c.Authorization, &c.Authentication)
        // TODO: handle stoppedCh returned by c.SecureServing.Serve
        if _, err := c.SecureServing.Serve(handler, 0, stopCh); err != nil {
            return err
        }
    }
    if c.InsecureServing != nil {
        unsecuredMux = genericcontrollermanager.NewBaseHandler(&c.ComponentConfig.Generic.Debugging, checks...)
        insecureSuperuserAuthn := server.AuthenticationInfo{Authenticator: &server.InsecureSuperuser{}}
        handler := genericcontrollermanager.BuildHandlerChain(unsecuredMux, nil, &insecureSuperuserAuthn)
        if err := c.InsecureServing.Serve(handler, 0, stopCh); err != nil {
            return err
        }
    }

    run := func(ctx context.Context) {
        // 判断使用哪种client，simple client or dynamic client
        rootClientBuilder := controller.SimpleControllerClientBuilder{
            ClientConfig: c.Kubeconfig,
        }
        var clientBuilder controller.ControllerClientBuilder
        if c.ComponentConfig.KubeCloudShared.UseServiceAccountCredentials {
            if len(c.ComponentConfig.SAController.ServiceAccountKeyFile) == 0 {
                // It's possible another controller process is creating the tokens for us.
                // If one isn't, we'll timeout and exit when our client builder is unable to create the tokens.
                klog.Warningf("--use-service-account-credentials was specified without providing a --service-account-private-key-file")
            }

            if shouldTurnOnDynamicClient(c.Client) {
                klog.V(1).Infof("using dynamic client builder")
                //Dynamic builder will use TokenRequest feature and refresh service account token periodically
                clientBuilder = controller.NewDynamicClientBuilder(
                    restclient.AnonymousClientConfig(c.Kubeconfig),
                    c.Client.CoreV1(),
                    "kube-system")
            } else {
                klog.V(1).Infof("using legacy client builder")
                clientBuilder = controller.SAControllerClientBuilder{
                    ClientConfig:         restclient.AnonymousClientConfig(c.Kubeconfig),
                    CoreClient:           c.Client.CoreV1(),
                    AuthenticationClient: c.Client.AuthenticationV1(),
                    Namespace:            "kube-system",
                }
            }
        } else {
            clientBuilder = rootClientBuilder
        }
        controllerContext, err := CreateControllerContext(c, rootClientBuilder, clientBuilder, ctx.Done())
        if err != nil {
            klog.Fatalf("error building controller context: %v", err)
        }
        saTokenControllerInitFunc := serviceAccountTokenControllerStarter{rootClientBuilder: rootClientBuilder}.startServiceAccountTokenController

        if err := StartControllers(controllerContext, saTokenControllerInitFunc, NewControllerInitializers(controllerContext.LoopMode), unsecuredMux); err != nil {
            klog.Fatalf("error starting controllers: %v", err)
        }

        // Start informers,开始接收相应的事件
        controllerContext.InformerFactory.Start(controllerContext.Stop)
        controllerContext.ObjectOrMetadataInformerFactory.Start(controllerContext.Stop)
        close(controllerContext.InformersStarted)

        select {}
    }

    if !c.ComponentConfig.Generic.LeaderElection.LeaderElect {
        run(context.TODO())
        panic("unreachable")
    }

    id, err := os.Hostname()
    if err != nil {
        return err
    }

    // add a uniquifier so that two processes on the same host don't accidentally both become active
    id = id + "_" + string(uuid.NewUUID())

    rl, err := resourcelock.New(c.ComponentConfig.Generic.LeaderElection.ResourceLock,
        c.ComponentConfig.Generic.LeaderElection.ResourceNamespace,
        c.ComponentConfig.Generic.LeaderElection.ResourceName,
        c.LeaderElectionClient.CoreV1(),
        c.LeaderElectionClient.CoordinationV1(),
        resourcelock.ResourceLockConfig{
            Identity:      id,
            EventRecorder: c.EventRecorder,
        })
    if err != nil {
        klog.Fatalf("error creating lock: %v", err)
    }

    leaderelection.RunOrDie(context.TODO(), leaderelection.LeaderElectionConfig{
        Lock:          rl,
        LeaseDuration: c.ComponentConfig.Generic.LeaderElection.LeaseDuration.Duration,
        RenewDeadline: c.ComponentConfig.Generic.LeaderElection.RenewDeadline.Duration,
        RetryPeriod:   c.ComponentConfig.Generic.LeaderElection.RetryPeriod.Duration,
        Callbacks: leaderelection.LeaderCallbacks{
            OnStartedLeading: run,
            OnStoppedLeading: func() {
                klog.Fatalf("leaderelection lost")
            },
        },
        WatchDog: electionChecker,
        Name:     "kube-controller-manager",
    })
    panic("unreachable")
}
```

再看下StartControllers和NewControllerInitializers 函数的具体实现，就能看出它是如何把每个controller都调用起来的。

```go
func StartControllers(ctx ControllerContext, startSATokenController InitFunc, controllers map[string]InitFunc, unsecuredMux *mux.PathRecorderMux) error {
    // Always start the SA token controller first using a full-power client, since it needs to mint tokens for the rest
    // If this fails, just return here and fail since other controllers won't be able to get credentials.
    if _, _, err := startSATokenController(ctx); err != nil {
        return err
    }

    // Initialize the cloud provider with a reference to the clientBuilder only after token controller
    // has started in case the cloud provider uses the client builder.
    if ctx.Cloud != nil {
        ctx.Cloud.Initialize(ctx.ClientBuilder, ctx.Stop)
    }

    for controllerName, initFn := range controllers {
        if !ctx.IsControllerEnabled(controllerName) {
            klog.Warningf("%q is disabled", controllerName)
            continue
        }

        time.Sleep(wait.Jitter(ctx.ComponentConfig.Generic.ControllerStartInterval.Duration, ControllerStartJitter))

        klog.V(1).Infof("Starting %q", controllerName)
        // 这里的initFn 就是NewControllerInitializers 函数返回的每个InitFunc，也就是每个资源的startXXXController函数。
        debugHandler, started, err := initFn(ctx)
        if err != nil {
            klog.Errorf("Error starting %q", controllerName)
            return err
        }
        if !started {
            klog.Warningf("Skipping %q", controllerName)
            continue
        }
        if debugHandler != nil && unsecuredMux != nil {
            basePath := "/debug/controllers/" + controllerName
            unsecuredMux.UnlistedHandle(basePath, http.StripPrefix(basePath, debugHandler))
            unsecuredMux.UnlistedHandlePrefix(basePath+"/", http.StripPrefix(basePath, debugHandler))
        }
        klog.Infof("Started %q", controllerName)
    }

    return nil
}

// NewControllerInitializers is a public map of named controller groups (you can start more than one in an init func)
// paired to their InitFunc.  This allows for structured downstream composition and subdivision.
func NewControllerInitializers(loopMode ControllerLoopMode) map[string]InitFunc {
    controllers := map[string]InitFunc{}
    controllers["endpoint"] = startEndpointController
    controllers["endpointslice"] = startEndpointSliceController
    controllers["replicationcontroller"] = startReplicationController
    controllers["podgc"] = startPodGCController
    controllers["resourcequota"] = startResourceQuotaController
    controllers["namespace"] = startNamespaceController
    controllers["serviceaccount"] = startServiceAccountController
    controllers["garbagecollector"] = startGarbageCollectorController
    controllers["daemonset"] = startDaemonSetController
    controllers["job"] = startJobController
    controllers["deployment"] = startDeploymentController
    controllers["replicaset"] = startReplicaSetController
    controllers["horizontalpodautoscaling"] = startHPAController
    controllers["disruption"] = startDisruptionController
    controllers["statefulset"] = startStatefulSetController
    controllers["cronjob"] = startCronJobController
    controllers["csrsigning"] = startCSRSigningController
    controllers["csrapproving"] = startCSRApprovingController
    controllers["csrcleaner"] = startCSRCleanerController
    controllers["ttl"] = startTTLController
    controllers["bootstrapsigner"] = startBootstrapSignerController
    controllers["tokencleaner"] = startTokenCleanerController
    controllers["nodeipam"] = startNodeIpamController
    controllers["nodelifecycle"] = startNodeLifecycleController
    if loopMode == IncludeCloudLoops {
        controllers["service"] = startServiceController
        controllers["route"] = startRouteController
        controllers["cloud-node-lifecycle"] = startCloudNodeLifecycleController
        // TODO: volume controller into the IncludeCloudLoops only set.
    }
    controllers["persistentvolume-binder"] = startPersistentVolumeBinderController
    controllers["attachdetach"] = startAttachDetachController
    controllers["persistentvolume-expander"] = startVolumeExpandController
    controllers["clusterrole-aggregation"] = startClusterRoleAggregrationController
    controllers["pvc-protection"] = startPVCProtectionController
    controllers["pv-protection"] = startPVProtectionController
    controllers["ttl-after-finished"] = startTTLAfterFinishedController
    controllers["root-ca-cert-publisher"] = startRootCACertPublisher

    return controllers
}
```

好了，下面在以deployment为例，进行深入分析。我们再进入[startDeploymentController](https://github.com/kubernetes/kubernetes/blob/release-1.18/cmd/kube-controller-manager/app/apps.go#L82:6)看下这里，到底做了什么。

```go
func startDeploymentController(ctx ControllerContext) (http.Handler, bool, error) {
    if !ctx.AvailableResources[schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}] {
        return nil, false, nil
    }
    dc, err := deployment.NewDeploymentController(
        ctx.InformerFactory.Apps().V1().Deployments(),
        ctx.InformerFactory.Apps().V1().ReplicaSets(),
        ctx.InformerFactory.Core().V1().Pods(),
        ctx.ClientBuilder.ClientOrDie("deployment-controller"),
    )
    if err != nil {
        return nil, true, fmt.Errorf("error creating Deployment controller: %v", err)
    }
    go dc.Run(int(ctx.ComponentConfig.DeploymentController.ConcurrentDeploymentSyncs), ctx.Stop)
    return nil, true, nil
}
```

大家肯定知道，现在创建一个deployment,它将会自己创建对应的replicaset和对应的desired个数的pod。所以deployment的controller 同时要实现deployment，replicaset和pod的controller。从上面的NewDeploymentController函数中，也确实能看到这点。

再进去[NewDeploymentController](https://github.com/kubernetes/kubernetes/blob/release-1.18/pkg/controller/deployment/deployment_controller.go#L101:6)，看看它又是做了什么呢！

其实到了这里，如果大家有研究过k8s提供的client-go(大家可以参考这个官方例子[workqueue](https://github.com/kubernetes/client-go/blob/master/examples/workqueue/main.go))的话，就很容易看懂每个具体的controller了，因为它们都是按照同样的框架写的。

```go
// NewDeploymentController creates a new DeploymentController.
func NewDeploymentController(dInformer appsinformers.DeploymentInformer, rsInformer appsinformers.ReplicaSetInformer, podInformer coreinformers.PodInformer, client clientset.Interface) (*DeploymentController, error) {
    eventBroadcaster := record.NewBroadcaster()
    eventBroadcaster.StartLogging(klog.Infof)
    eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

    if client != nil && client.CoreV1().RESTClient().GetRateLimiter() != nil {
        if err := ratelimiter.RegisterMetricAndTrackRateLimiterUsage("deployment_controller", client.CoreV1().RESTClient().GetRateLimiter()); err != nil {
            return nil, err
        }
    }
    dc := &DeploymentController{
        client:        client,
        eventRecorder: eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "deployment-controller"}),
        queue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "deployment"),
    }
    dc.rsControl = controller.RealRSControl{
        KubeClient: client,
        Recorder:   dc.eventRecorder,
    }

    dInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    dc.addDeployment,
        UpdateFunc: dc.updateDeployment,
        // This will enter the sync loop and no-op, because the deployment has been deleted from the store.
        DeleteFunc: dc.deleteDeployment,
    })
    rsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        AddFunc:    dc.addReplicaSet,
        UpdateFunc: dc.updateReplicaSet,
        DeleteFunc: dc.deleteReplicaSet,
    })
    podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
        DeleteFunc: dc.deletePod,
    })

    dc.syncHandler = dc.syncDeployment
    dc.enqueueDeployment = dc.enqueue

    dc.dLister = dInformer.Lister()
    dc.rsLister = rsInformer.Lister()
    dc.podLister = podInformer.Lister()
    dc.dListerSynced = dInformer.Informer().HasSynced
    dc.rsListerSynced = rsInformer.Informer().HasSynced
    dc.podListerSynced = podInformer.Informer().HasSynced
    return dc, nil
}
```

这里就是通过重写资源的Add,Update,Delete 的事件处理逻辑，就能实现当资源产生这些事件时，这些handler就会被执行。但是由于这里实现了workqueue的机制，你将会发现，这里的handler都是往一个queue里面装，然后真正从queue里面取事件，然后执行的是syncHandler这个函数接口，在上面函数里，可以看到，它其实指向了dc.syncDeployment这个函数，也就是说，最后真正执行逻辑处理的是这个函数。

所以具体的处理逻辑在这个[syncDeployment](https://github.com/kubernetes/kubernetes/blob/release-1.18/pkg/controller/deployment/deployment_controller.go#L563:33)函数中。

```go
func (dc *DeploymentController) syncDeployment(key string) error {
    startTime := time.Now()
    klog.V(4).Infof("Started syncing deployment %q (%v)", key, startTime)
    defer func() {
        klog.V(4).Infof("Finished syncing deployment %q (%v)", key, time.Since(startTime))
    }()

    namespace, name, err := cache.SplitMetaNamespaceKey(key)
    if err != nil {
        return err
    }
    // 获取指定namespace 和name 的deployment
    deployment, err := dc.dLister.Deployments(namespace).Get(name)
    if errors.IsNotFound(err) {
        klog.V(2).Infof("Deployment %v has been deleted", key)
        return nil
    }
    if err != nil {
        return err
    }

    // Deep-copy otherwise we are mutating our cache.
    // TODO: Deep-copy only when needed.
    d := deployment.DeepCopy()

    everything := metav1.LabelSelector{}
    if reflect.DeepEqual(d.Spec.Selector, &everything) {
        dc.eventRecorder.Eventf(d, v1.EventTypeWarning, "SelectingAll", "This deployment is selecting all pods. A non-empty selector is required.")
        if d.Status.ObservedGeneration < d.Generation {
            d.Status.ObservedGeneration = d.Generation
            dc.client.AppsV1().Deployments(d.Namespace).UpdateStatus(context.TODO(), d, metav1.UpdateOptions{})
        }
        return nil
    }

    // 获取这个deployment下的replicaset
    rsList, err := dc.getReplicaSetsForDeployment(d)
    if err != nil {
        return err
    }
    // 列出deployment下replicaset产生的pod
    podMap, err := dc.getPodMapForDeployment(d, rsList)
    if err != nil {
        return err
    }

    if d.DeletionTimestamp != nil {
        return dc.syncStatusOnly(d, rsList)
    }

    // Update deployment conditions with an Unknown condition when pausing/resuming
    // a deployment. In this way, we can be sure that we won't timeout when a user
    // resumes a Deployment with a set progressDeadlineSeconds.
    if err = dc.checkPausedConditions(d); err != nil {
        return err
    }

    if d.Spec.Paused {
        return dc.sync(d, rsList)
    }

    // rollback is not re-entrant in case the underlying replica sets are updated with a new
    // revision so we should ensure that we won't proceed to update replica sets until we
    // make sure that the deployment has cleaned up its rollback spec in subsequent enqueues.
    if getRollbackTo(d) != nil {
        return dc.rollback(d, rsList)
    }

    // check 这里是不是需要scaling操作
    scalingEvent, err := dc.isScalingEvent(d, rsList)
    if err != nil {
        return err
    }
    if scalingEvent {
        // 进行同步更新操作
        return dc.sync(d, rsList)
    }

    switch d.Spec.Strategy.Type {
    case apps.RecreateDeploymentStrategyType:
        return dc.rolloutRecreate(d, rsList, podMap)
    case apps.RollingUpdateDeploymentStrategyType:
        return dc.rolloutRolling(d, rsList)
    }
    return fmt.Errorf("unexpected deployment strategy type: %s", d.Spec.Strategy.Type)
}
```

上面通过[isScalingEvent](https://github.com/kubernetes/kubernetes/blob/release-1.18/pkg/controller/deployment/sync.go#L529:33)判断是不是需要进行同步更新的操作，如果需要，则调用[sync](https://github.com/kubernetes/kubernetes/blob/release-1.18/pkg/controller/deployment/sync.go#L49) 进行同步。

```go
// isScalingEvent checks whether the provided deployment has been updated with a scaling event
// by looking at the desired-replicas annotation in the active replica sets of the deployment.
//
// rsList should come from getReplicaSetsForDeployment(d).
// podMap should come from getPodMapForDeployment(d, rsList).
func (dc *DeploymentController) isScalingEvent(d *apps.Deployment, rsList []*apps.ReplicaSet) (bool, error) {
    newRS, oldRSs, err := dc.getAllReplicaSetsAndSyncRevision(d, rsList, false)
    if err != nil {
        return false, err
    }
    allRSs := append(oldRSs, newRS)
    for _, rs := range controller.FilterActiveReplicaSets(allRSs) {
        desired, ok := deploymentutil.GetDesiredReplicasAnnotation(rs)
        if !ok {
            continue
        }
        if desired != *(d.Spec.Replicas) {
            return true, nil
        }
    }
    return false, nil
}

// sync is responsible for reconciling deployments on scaling events or when they
// are paused.
func (dc *DeploymentController) sync(d *apps.Deployment, rsList []*apps.ReplicaSet) error {
    newRS, oldRSs, err := dc.getAllReplicaSetsAndSyncRevision(d, rsList, false)
    if err != nil {
        return err
    }
    if err := dc.scale(d, newRS, oldRSs); err != nil {
        // If we get an error while trying to scale, the deployment will be requeued
        // so we can abort this resync
        return err
    }

    // Clean up the deployment when it's paused and no rollback is in flight.
    if d.Spec.Paused && getRollbackTo(d) == nil {
        if err := dc.cleanupDeployment(oldRSs, d); err != nil {
            return err
        }
    }

    allRSs := append(oldRSs, newRS)
    return dc.syncDeploymentStatus(allRSs, newRS, d)
}
```

最后就是调用[scale](https://github.com/kubernetes/kubernetes/blob/release-1.18/pkg/controller/deployment/sync.go#L298:33)进行scale up 或者scale down.

## Reference

[kubernetes](https://github.com/kubernetes/kubernetes)

[client-go](https://github.com/kubernetes/client-go)
