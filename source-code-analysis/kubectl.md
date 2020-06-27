# kubectl 源码分析(基于release-1.18 branch)

**NOTE: 由于代码篇幅太多，在分析的过程中会将不重要的部分删除，我将用//..................代替了.**

**NOTE: 再开始阅读这篇分析之前，如果有对cobra不是很了解的同学们,我建议先要大致去学一下cobra是怎么使用的。因为只有你对cobra有熟练或者一定的了解后，你再来阅读kubectl的源码，会比较轻松。这里我有一个简单的小例子可供参考[githelper](/playground/githelper)**

## 函数入口[kubectl.go](https://github.com/kubernetes/kubernetes/blob/release-1.18/cmd/kubectl/kubectl.go)

```go
func main() {
    rand.Seed(time.Now().UnixNano())

    command := cmd.NewDefaultKubectlCommand()

    // TODO: once we switch everything over to Cobra commands, we can go back to calling
    // cliflag.InitFlags() (by removing its pflag.Parse() call). For now, we have to set the
    // normalize func and add the go flag set by hand.
    pflag.CommandLine.SetNormalizeFunc(cliflag.WordSepNormalizeFunc)
    pflag.CommandLine.AddGoFlagSet(goflag.CommandLine)
    // cliflag.InitFlags()
    logs.InitLogs()
    defer logs.FlushLogs()

    if err := command.Execute(); err != nil {
        os.Exit(1)
    }
}
```

main函数一贯的简单明了，我们进入[NewDefaultKubectlCommand()](https://github.com/kubernetes/kubernetes/blob/release-1.18/pkg/kubectl/cmd/cmd.go#L300)函数探索一下。

```go
// NewDefaultKubectlCommand creates the `kubectl` command with default arguments
func NewDefaultKubectlCommand() *cobra.Command {
    return NewDefaultKubectlCommandWithArgs(NewDefaultPluginHandler(plugin.ValidPluginFilenamePrefixes), os.Args, os.Stdin, os.Stdout, os.Stderr)
}

// NewDefaultKubectlCommandWithArgs creates the `kubectl` command with arguments
func NewDefaultKubectlCommandWithArgs(pluginHandler PluginHandler, args []string, in io.Reader, out, errout io.Writer) *cobra.Command {
    cmd := NewKubectlCommand(in, out, errout)

    //..................

    return cmd
}

// NewKubectlCommand creates the `kubectl` command and its nested children.
func NewKubectlCommand(in io.Reader, out, err io.Writer) *cobra.Command {
    // Parent command to which all subcommands are added.
    // 1 创建root command
    cmds := &cobra.Command{
        Use:   "kubectl",
        Short: i18n.T("kubectl controls the Kubernetes cluster manager"),
        Long: templates.LongDesc(`
      kubectl controls the Kubernetes cluster manager.
      Find more information at:
            https://kubernetes.io/docs/reference/kubectl/overview/`),
        Run: runHelp,
        // Hook before and after Run initialize and write profiles to disk,
        // respectively.
        PersistentPreRunE: func(*cobra.Command, []string) error {
            return initProfiling()
        },
        PersistentPostRunE: func(*cobra.Command, []string) error {
            return flushProfiling()
        },
        BashCompletionFunction: bashCompletionFunc,
    }
    
    //...............................

    // 2. 创建了个Factory.
    f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

    // Sending in 'nil' for the getLanguageFn() results in using
    // the LANG environment variable.
    //
    // TODO: Consider adding a flag or file preference for setting
    // the language, instead of just loading from the LANG env. variable.
    i18n.LoadTranslations("kubectl", nil)

    // From this point and forward we get warnings on flags that contain "_" separators
    cmds.SetGlobalNormalizationFunc(cliflag.WarnWordSepNormalizeFunc)

    ioStreams := genericclioptions.IOStreams{In: in, Out: out, ErrOut: err}

    //3. 将7大类的command 存入到group里面
    groups := templates.CommandGroups{
        {
            Message: "Basic Commands (Beginner):",
            Commands: []*cobra.Command{
                create.NewCmdCreate(f, ioStreams),
                expose.NewCmdExposeService(f, ioStreams),
                run.NewCmdRun(f, ioStreams),
                set.NewCmdSet(f, ioStreams),
            },
        },
        {
            Message: "Basic Commands (Intermediate):",
            Commands: []*cobra.Command{
                explain.NewCmdExplain("kubectl", f, ioStreams),
                get.NewCmdGet("kubectl", f, ioStreams),
                edit.NewCmdEdit(f, ioStreams),
                delete.NewCmdDelete(f, ioStreams),
            },
        },
        {
            Message: "Deploy Commands:",
            Commands: []*cobra.Command{
                rollout.NewCmdRollout(f, ioStreams),
                scale.NewCmdScale(f, ioStreams),
                autoscale.NewCmdAutoscale(f, ioStreams),
            },
        },
        {
            Message: "Cluster Management Commands:",
            Commands: []*cobra.Command{
                certificates.NewCmdCertificate(f, ioStreams),
                clusterinfo.NewCmdClusterInfo(f, ioStreams),
                top.NewCmdTop(f, ioStreams),
                drain.NewCmdCordon(f, ioStreams),
                drain.NewCmdUncordon(f, ioStreams),
                drain.NewCmdDrain(f, ioStreams),
                taint.NewCmdTaint(f, ioStreams),
            },
        },
        {
            Message: "Troubleshooting and Debugging Commands:",
            Commands: []*cobra.Command{
                describe.NewCmdDescribe("kubectl", f, ioStreams),
                logs.NewCmdLogs(f, ioStreams),
                attach.NewCmdAttach(f, ioStreams),
                cmdexec.NewCmdExec(f, ioStreams),
                portforward.NewCmdPortForward(f, ioStreams),
                proxy.NewCmdProxy(f, ioStreams),
                cp.NewCmdCp(f, ioStreams),
                auth.NewCmdAuth(f, ioStreams),
            },
        },
        {
            Message: "Advanced Commands:",
            Commands: []*cobra.Command{
                diff.NewCmdDiff(f, ioStreams),
                apply.NewCmdApply("kubectl", f, ioStreams),
                patch.NewCmdPatch(f, ioStreams),
                replace.NewCmdReplace(f, ioStreams),
                wait.NewCmdWait(f, ioStreams),
                convert.NewCmdConvert(f, ioStreams),
                kustomize.NewCmdKustomize(ioStreams),
            },
        },
        {
            Message: "Settings Commands:",
            Commands: []*cobra.Command{
                label.NewCmdLabel(f, ioStreams),
                annotate.NewCmdAnnotate("kubectl", f, ioStreams),
                completion.NewCmdCompletion(ioStreams.Out, ""),
            },
        },
    }
    groups.Add(cmds)

    filters := []string{"options"}

    // Hide the "alpha" subcommand if there are no alpha commands in this build.
    alpha := cmdpkg.NewCmdAlpha(f, ioStreams)
    if !alpha.HasSubCommands() {
        filters = append(filters, alpha.Name())
    }

    templates.ActsAsRootCommand(cmds, filters, groups...)

    for name, completion := range bashCompletionFlags {
        if cmds.Flag(name) != nil {
            if cmds.Flag(name).Annotations == nil {
                cmds.Flag(name).Annotations = map[string][]string{}
            }
            cmds.Flag(name).Annotations[cobra.BashCompCustom] = append(
                cmds.Flag(name).Annotations[cobra.BashCompCustom],
                completion,
            )
        }
    }

    //4. 将第八大类(其他)添加到root command里面
    cmds.AddCommand(alpha)
    cmds.AddCommand(cmdconfig.NewCmdConfig(f, clientcmd.NewDefaultPathOptions(), ioStreams))
    cmds.AddCommand(plugin.NewCmdPlugin(f, ioStreams))
    cmds.AddCommand(version.NewCmdVersion(f, ioStreams))
    cmds.AddCommand(apiresources.NewCmdAPIVersions(f, ioStreams))
    cmds.AddCommand(apiresources.NewCmdAPIResources(f, ioStreams))
    cmds.AddCommand(options.NewCmdOptions(ioStreams.Out))

    return cmds
}
```

从上面函数调用关系来看，最终是在[NewKubectlCommand](https://github.com/kubernetes/kubernetes/blob/release-1.18/pkg/kubectl/cmd/cmd.go#L430)干了一堆活。下面我们来分析一下它具体做了些什么呢！

1. 创建一个root command.

2. 创建了个Factory.

3. 将下面7大类的command存入到group里.

    1. Basic Commands (Beginner)
    2. Basic Commands (Intermediate)
    3. Deploy Commands
    4. Cluster Management Commands
    5. Troubleshooting and Debugging Commands
    6. Advanced Commands
    7. Settings Commands

4. 将第八大类(其他)添加到root command里面.

好了，至此各种命令成功加入到root命令。下面我们将举一个命令为例，再深入分析一下。不过在此之前，我们要来看下第二步NewFactory这个函数，细心的同学们应该有察觉到所有的命令中都传入了这个factory。所以可以看出这个工厂很是重要。

ok, 我们一起来看看那这个[NewFactory](https://github.com/kubernetes/kubernetes/blob/release-1.18/staging/src/k8s.io/kubectl/pkg/cmd/util/factory_client_access.go#L50)函数创建的是什么实例呢！

```go

type factoryImpl struct {
    clientGetter genericclioptions.RESTClientGetter

    // openAPIGetter loads and caches openapi specs
    openAPIGetter openAPIGetter
}

func NewFactory(clientGetter genericclioptions.RESTClientGetter) Factory {
    if clientGetter == nil {
        panic("attempt to instantiate client_access_factory with nil clientGetter")
    }

    f := &factoryImpl{
        clientGetter: clientGetter,
    }

    return f
}

```

上面那个NewFactory函数 创建了个factoryImpl实例，但是返回的是个Factory inferface。 其实在这里它是实现了个简单工厂模式将具体的实现隐藏了起来。好了，那么我们就来看看这个Factory Interface有哪些接口.

```go
type Factory interface {
    genericclioptions.RESTClientGetter

    // DynamicClient returns a dynamic client ready for use
    DynamicClient() (dynamic.Interface, error)

    // KubernetesClientSet gives you back an external clientset
    KubernetesClientSet() (*kubernetes.Clientset, error)

    // Returns a RESTClient for accessing Kubernetes resources or an error.
    RESTClient() (*restclient.RESTClient, error)

    // NewBuilder returns an object that assists in loading objects from both disk and the server
    // and which implements the common patterns for CLI interactions with generic resources.
    NewBuilder() *resource.Builder

    // Returns a RESTClient for working with the specified RESTMapping or an error. This is intended
    // for working with arbitrary resources and is not guaranteed to point to a Kubernetes APIServer.
    ClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error)
    // Returns a RESTClient for working with Unstructured objects.
    UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error)

    // Returns a schema that can validate objects stored on disk.
    Validator(validate bool) (validation.Schema, error)
    // OpenAPISchema returns the schema openapi schema definition
    OpenAPISchema() (openapi.Resources, error)
}
```

在这里，我们就不分析factoryImpl实例是具体怎么实现这些接口的了。我们下面要以[create命令](https://github.com/kubernetes/kubernetes/blob/release-1.18/staging/src/k8s.io/kubectl/pkg/cmd/create/create.go#L98)为例深入的分析下它是怎么工作的。

```go
// NewCmdCreate returns new initialized instance of create sub command
func NewCmdCreate(f cmdutil.Factory, ioStreams genericclioptions.IOStreams) *cobra.Command {
    o := NewCreateOptions(ioStreams)

    //1. 创建了个create命令
    cmd := &cobra.Command{
        Use:                   "create -f FILENAME",
        DisableFlagsInUseLine: true,
        Short:                 i18n.T("Create a resource from a file or from stdin."),
        Long:                  createLong,
        Example:               createExample,
        Run: func(cmd *cobra.Command, args []string) {
            if cmdutil.IsFilenameSliceEmpty(o.FilenameOptions.Filenames, o.FilenameOptions.Kustomize) {
                ioStreams.ErrOut.Write([]byte("Error: must specify one of -f and -k\n\n"))
                defaultRunFunc := cmdutil.DefaultSubCommandRun(ioStreams.ErrOut)
                defaultRunFunc(cmd, args)
                return
            }
            cmdutil.CheckErr(o.Complete(f, cmd))
            cmdutil.CheckErr(o.ValidateArgs(cmd, args))
            cmdutil.CheckErr(o.RunCreate(f, cmd))
        },
    }

    //2. 绑定一些flags
    // bind flag structs
    o.RecordFlags.AddFlags(cmd)

    usage := "to use to create the resource"
    cmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
    cmdutil.AddValidateFlags(cmd)
    cmd.Flags().BoolVar(&o.EditBeforeCreate, "edit", o.EditBeforeCreate, "Edit the API resource before creating")
    cmd.Flags().Bool("windows-line-endings", runtime.GOOS == "windows",
        "Only relevant if --edit=true. Defaults to the line ending native to your platform.")
    cmdutil.AddApplyAnnotationFlags(cmd)
    cmdutil.AddDryRunFlag(cmd)
    cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
    cmd.Flags().StringVar(&o.Raw, "raw", o.Raw, "Raw URI to POST to the server.  Uses the transport specified by the kubeconfig file.")

    o.PrintFlags.AddFlags(cmd)

    // 3. 创建一些子命令并添加到create命令中
    // create subcommands
    cmd.AddCommand(NewCmdCreateNamespace(f, ioStreams))
    cmd.AddCommand(NewCmdCreateQuota(f, ioStreams))
    cmd.AddCommand(NewCmdCreateSecret(f, ioStreams))
    cmd.AddCommand(NewCmdCreateConfigMap(f, ioStreams))
    cmd.AddCommand(NewCmdCreateServiceAccount(f, ioStreams))
    cmd.AddCommand(NewCmdCreateService(f, ioStreams))
    cmd.AddCommand(NewCmdCreateDeployment(f, ioStreams))
    cmd.AddCommand(NewCmdCreateClusterRole(f, ioStreams))
    cmd.AddCommand(NewCmdCreateClusterRoleBinding(f, ioStreams))
    cmd.AddCommand(NewCmdCreateRole(f, ioStreams))
    cmd.AddCommand(NewCmdCreateRoleBinding(f, ioStreams))
    cmd.AddCommand(NewCmdCreatePodDisruptionBudget(f, ioStreams))
    cmd.AddCommand(NewCmdCreatePriorityClass(f, ioStreams))
    cmd.AddCommand(NewCmdCreateJob(f, ioStreams))
    cmd.AddCommand(NewCmdCreateCronJob(f, ioStreams))
    return cmd
}
```

从上面代码中可以看到，此函数做了

1. 创建了个create命令
2. 绑定一些flags
3. 创建一些子命令并添加到create命令中

下面我们看看当你调用kubectl create -f xxx.yaml时具体是谁做了什么呢。分析到这里我想同学们都已经知道了，那这里就是这个[RunCreate](https://github.com/kubernetes/kubernetes/blob/release-1.18/staging/src/k8s.io/kubectl/pkg/cmd/create/create.go#L224)函数的工作了。

```go
// RunCreate performs the creation
func (o *CreateOptions) RunCreate(f cmdutil.Factory, cmd *cobra.Command) error {
    // raw only makes sense for a single file resource multiple objects aren't likely to do what you want.
    // the validator enforces this, so
    //判断是否是Raw string ，如果是就直接http post过去创建了
    if len(o.Raw) > 0 {
        restClient, err := f.RESTClient()
        if err != nil {
            return err
        }
        return rawhttp.RawPost(restClient, o.IOStreams, o.Raw, o.FilenameOptions.Filenames[0])
    }

    // 判断是否在创建之前要进行编辑
    if o.EditBeforeCreate {
        return RunEditOnCreate(f, o.PrintFlags, o.RecordFlags, o.IOStreams, cmd, &o.FilenameOptions)
    }
    schema, err := f.Validator(cmdutil.GetFlagBool(cmd, "validate"))
    if err != nil {
        return err
    }

    cmdNamespace, enforceNamespace, err := f.ToRawKubeConfigLoader().Namespace()
    if err != nil {
        return err
    }

    //创建一个Builder 实例，并初始化一些数据，在调用Do函数去嵌套Visitor
    r := f.NewBuilder().
        Unstructured().
        Schema(schema).
        ContinueOnError().
        NamespaceParam(cmdNamespace).DefaultNamespace().
        FilenameParam(enforceNamespace, &o.FilenameOptions).
        LabelSelectorParam(o.Selector).
        Flatten().
        Do()
    err = r.Err()
    if err != nil {
        return err
    }

    count := 0
    //调用上面builder.Do 返回的Result实例的Visit函数，执行创建资源操作
    err = r.Visit(func(info *resource.Info, err error) error {
        //....................................

        // 这里要判断下是不是DryRun, 如果是的 就只打印object内容，不会创建资源
        if o.DryRunStrategy != cmdutil.DryRunClient {
            if o.DryRunStrategy == cmdutil.DryRunServer {
                if err := o.DryRunVerifier.HasSupport(info.Mapping.GroupVersionKind); err != nil {
                    return cmdutil.AddSourceToErr("creating", info.Source, err)
                }
            }
            // 创建资源
            obj, err := resource.
                NewHelper(info.Client, info.Mapping).
                DryRun(o.DryRunStrategy == cmdutil.DryRunServer).
                Create(info.Namespace, true, info.Object)
            if err != nil {
                return cmdutil.AddSourceToErr("creating", info.Source, err)
            }
            //更新info
            info.Refresh(obj, true)
        }

        count++

        return o.PrintObj(info.Object)
    })
    if err != nil {
        return err
    }
    if count == 0 {
        return fmt.Errorf("no objects passed to create")
    }
    return nil
}

```

这个函数里面，在调用Builder创建Result的时候用了Visitor设计模式。并且它不是简单的使用了visitor，它写的较为复杂，它现实了一堆嵌套的visitor。为了更好理解，可以先参考小例子[MultipleNestedVisitor](https://github.com/Luffy110/golang-design-pattern/tree/master/24_nested_visitor)，个人觉得这部分也是kubectl中最难理解的部分.**(这里我要为大家推荐一本源码剖析的书，kubernetes源码剖析-郑东旭， 因为我也是看了这本书里面的讲解，才更深的理解了这部分。这本书不是完全的源码解析。目前只包括了master的分析。据说Node的部分会在第二本中分析)**. 好了闲话不多说。 让我们回归正题，慢慢来分析。

首先我们来分析分析下面这段代码做了些什么！这里很关键！因为它会决定后面我们调用Visit时，嵌套了那些Visitor在里面。我们将一个一个来分析。

```go
    r := f.NewBuilder().
        Unstructured().
        Schema(schema).
        ContinueOnError().
        NamespaceParam(cmdNamespace).DefaultNamespace().
        FilenameParam(enforceNamespace, &o.FilenameOptions).
        LabelSelectorParam(o.Selector).
        Flatten().
        Do()
 
```

这里我们想同学们都还记得这个f的实例是谁吧。没错，是factoryImpl。就是上面提到的用简单工厂模式生产出来的。既然我们知道了f是谁了。那我们就来看看[factoryImpl.NewBuilder()](https://github.com/kubernetes/kubernetes/blob/release-1.18/staging/src/k8s.io/kubectl/pkg/cmd/util/factory_client_access.go#L95)函数

```go
// NewBuilder returns a new resource builder for structured api objects.
func (f *factoryImpl) NewBuilder() *resource.Builder {
    return resource.NewBuilder(f.clientGetter)
}
```

原来就是把resource.NewBuilder分装了一层。那么我们来看看[resource.NewBuilder](https://github.com/kubernetes/kubernetes/blob/release-1.18/staging/src/k8s.io/cli-runtime/pkg/resource/builder.go#L181).

```go
func NewBuilder(restClientGetter RESTClientGetter) *Builder {
    categoryExpanderFn := func() (restmapper.CategoryExpander, error) {
        discoveryClient, err := restClientGetter.ToDiscoveryClient()
        if err != nil {
            return nil, err
        }
        return restmapper.NewDiscoveryCategoryExpander(discoveryClient), err
    }
    //创建了个Builder实例
    return newBuilder(
        restClientGetter.ToRESTConfig,
        (&cachingRESTMapperFunc{delegate: restClientGetter.ToRESTMapper}).ToRESTMapper,
        (&cachingCategoryExpanderFunc{delegate: categoryExpanderFn}).ToCategoryExpander,
    )
}
```

从上面code 我们看到它实例化了个Builder实例。下面给出Builder的具体结构内容。

```go
// Builder provides convenience functions for taking arguments and parameters
// from the command line and converting them to a list of resources to iterate
// over using the Visitor interface.
type Builder struct {
    categoryExpanderFn CategoryExpanderFunc

    // mapper is set explicitly by resource builders
    mapper *mapper

    // clientConfigFn is a function to produce a client, *if* you need one
    clientConfigFn ClientConfigFunc

    restMapperFn RESTMapperFunc

    // objectTyper is statically determinant per-command invocation based on your internal or unstructured choice
    // it does not ever need to rely upon discovery.
    objectTyper runtime.ObjectTyper

    // codecFactory describes which codecs you want to use
    negotiatedSerializer runtime.NegotiatedSerializer

    // local indicates that we cannot make server calls
    local bool

    errs []error

    paths  []Visitor
    stream bool
    dir    bool

    labelSelector     *string
    fieldSelector     *string
    selectAll         bool
    limitChunks       int64
    requestTransforms []RequestTransform

    resources []string

    namespace    string
    allNamespace bool
    names        []string

    resourceTuples []resourceTuple

    defaultNamespace bool
    requireNamespace bool

    flatten bool
    latest  bool

    requireObject bool

    singleResourceType bool
    continueOnError    bool

    singleItemImplied bool

    export bool

    schema ContentValidator

    // fakeClientFn is used for testing
    fakeClientFn FakeClientFunc
}
```

我们再来一个一个分析刚刚上面那个NewBuilder后面的每个函数。这些函数大多数都是在给Builder初始化一些变量值，也有部分是再构造嵌套的Visitor了。这些赋值都很重要，因为后面会根据这些变量值进行构建嵌套的Visitor, 所以也就是说，会影响到后面的Visit函数的具体行为。

先来看看Unstructured()函数

```go
// Unstructured updates the builder so that it will request and send unstructured
// objects. Unstructured objects preserve all fields sent by the server in a map format
// based on the object's JSON structure which means no data is lost when the client
// reads and then writes an object. Use this mode in preference to Internal unless you
// are working with Go types directly.
func (b *Builder) Unstructured() *Builder {
    if b.mapper != nil {
        b.errs = append(b.errs, fmt.Errorf("another mapper was already selected, cannot use unstructured types"))
        return b
    }
    b.objectTyper = unstructuredscheme.NewUnstructuredObjectTyper()
    //创建个mapper，赋值给了builder 的mapper成员
    b.mapper = &mapper{
        localFn:      b.isLocal,
        restMapperFn: b.restMapperFn,
        clientFn:     b.getClient,
        decoder:      &metadataValidatingDecoder{unstructured.UnstructuredJSONScheme},
    }

    return b
}
```

上面这个函数创建了个mapper和objectTyper赋值给了Builder实例。

再来看看Schema()函数

```go
func (b *Builder) Schema(schema ContentValidator) *Builder {
    b.schema = schema
    return b
}
```

就是传入一个schema赋值给了builder的schema。

下面是ContinueOnError()函数

```go
// ContinueOnError will attempt to load and visit as many objects as possible, even if some visits
// return errors or some objects cannot be loaded. The default behavior is to terminate after
// the first error is returned from a VisitorFunc.
func (b *Builder) ContinueOnError() *Builder {
    b.continueOnError = true
    return b
}
```

这个函数将builder的continueOnError设置为了true。

下面是NamespaceParam()和DefaultNamespace()函数

```go
// NamespaceParam accepts the namespace that these resources should be
// considered under from - used by DefaultNamespace() and RequireNamespace()
func (b *Builder) NamespaceParam(namespace string) *Builder {
    b.namespace = namespace
    return b
}

// DefaultNamespace instructs the builder to set the namespace value for any object found
// to NamespaceParam() if empty.
func (b *Builder) DefaultNamespace() *Builder {
    b.defaultNamespace = true
    return b
}
```

这里传入了namespace 赋值给了builder的namespace，并将builder的defaultNamespace设置为了true。

下面是FilenameParam()函数

```go
// FilenameParam groups input in two categories: URLs and files (files, directories, STDIN)
// If enforceNamespace is false, namespaces in the specs will be allowed to
// override the default namespace. If it is true, namespaces that don't match
// will cause an error.
// If ContinueOnError() is set prior to this method, objects on the path that are not
// recognized will be ignored (but logged at V(2)).
func (b *Builder) FilenameParam(enforceNamespace bool, filenameOptions *FilenameOptions) *Builder {
    // 校验一下filenameOptions是否正确
    if errs := filenameOptions.validate(); len(errs) > 0 {
        b.errs = append(b.errs, errs...)
        return b
    }
    recursive := filenameOptions.Recursive
    paths := filenameOptions.Filenames
    // 循环的处理传入的文件
    for _, s := range paths {
        switch {
        case s == "-":
            b.Stdin()
        case strings.Index(s, "http://") == 0 || strings.Index(s, "https://") == 0:
            url, err := url.Parse(s)
            if err != nil {
                b.errs = append(b.errs, fmt.Errorf("the URL passed to filename %q is not valid: %v", s, err))
                continue
            }
            b.URL(defaultHttpGetAttempts, url)
        default:
            if !recursive {
                b.singleItemImplied = true
            }
            b.Path(recursive, s)
        }
    }
    //判断是否是使用Kustomize的方式创建的。
    if filenameOptions.Kustomize != "" {
        b.paths = append(b.paths, &KustomizeVisitor{filenameOptions.Kustomize,
            NewStreamVisitor(nil, b.mapper, filenameOptions.Kustomize, b.schema)})
    }

    if enforceNamespace {
        b.RequireNamespace()
    }

    return b
}

// Path accepts a set of paths that may be files, directories (all can containing
// one or more resources). Creates a FileVisitor for each file and then each
// FileVisitor is streaming the content to a StreamVisitor. If ContinueOnError() is set
// prior to this method being called, objects on the path that are unrecognized will be
// ignored (but logged at V(2)).
func (b *Builder) Path(recursive bool, paths ...string) *Builder {
    for _, p := range paths {
        _, err := os.Stat(p)
        if os.IsNotExist(err) {
            b.errs = append(b.errs, fmt.Errorf("the path %q does not exist", p))
            continue
        }
        if err != nil {
            b.errs = append(b.errs, fmt.Errorf("the path %q cannot be accessed: %v", p, err))
            continue
        }

        visitors, err := ExpandPathsToFileVisitors(b.mapper, p, recursive, FileExtensions, b.schema)
        if err != nil {
            b.errs = append(b.errs, fmt.Errorf("error reading %q: %v", p, err))
        }
        if len(visitors) > 1 {
            b.dir = true
        }

        b.paths = append(b.paths, visitors...)
    }
    if len(b.paths) == 0 && len(b.errs) == 0 {
        b.errs = append(b.errs, fmt.Errorf("error reading %v: recognized file extensions are %v", paths, FileExtensions))
    }
    return b
}

// ExpandPathsToFileVisitors will return a slice of FileVisitors that will handle files from the provided path.
// After FileVisitors open the files, they will pass an io.Reader to a StreamVisitor to do the reading. (stdin
// is also taken care of). Paths argument also accepts a single file, and will return a single visitor
func ExpandPathsToFileVisitors(mapper *mapper, paths string, recursive bool, extensions []string, schema ContentValidator) ([]Visitor, error) {
    var visitors []Visitor
    err := filepath.Walk(paths, func(path string, fi os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if fi.IsDir() {
            if path != paths && !recursive {
                return filepath.SkipDir
            }
            return nil
        }
        // Don't check extension if the filepath was passed explicitly
        if path != paths && ignoreFile(path, extensions) {
            return nil
        }

        //构造了一个FileVisitor并创建了对应的StreamVisitor
        visitor := &FileVisitor{
            Path:          path,
            StreamVisitor: NewStreamVisitor(nil, mapper, path, schema),
        }

        visitors = append(visitors, visitor)
        return nil
    })

    if err != nil {
        return nil, err
    }
    return visitors, nil
}

// RequireNamespace instructs the builder to set the namespace value for any object found
// to NamespaceParam() if empty, and if the value on the resource does not match
// NamespaceParam() an error will be returned.
func (b *Builder) RequireNamespace() *Builder {
    b.requireNamespace = true
    return b
}
```

上面这个FilenameParam函数就是判断create命令的执行内容从哪里来的，并做相应的处理。

1. 从标准输入
2. 通过URL Visitor去读取创建内容。
3. 为每个Path创建一个FileVisitor，传入给builder.path。这里每个FileVisitor里包含一个path和一个StreamVisitor。
4. 通过Kustomize，则创建一个KustomizeVisitor， 并传入了一个StreamVisitor。

下面是LabelSelectorParam()和LabelSelector()函数

```go
// LabelSelectorParam defines a selector that should be applied to the object types to load.
// This will not affect files loaded from disk or URL. If the parameter is empty it is
// a no-op - to select all resources invoke `b.LabelSelector(labels.Everything.String)`.
func (b *Builder) LabelSelectorParam(s string) *Builder {
    selector := strings.TrimSpace(s)
    if len(selector) == 0 {
        return b
    }
    if b.selectAll {
        b.errs = append(b.errs, fmt.Errorf("found non-empty label selector %q with previously set 'all' parameter. ", s))
        return b
    }
    return b.LabelSelector(selector)
}

// LabelSelector accepts a selector directly and will filter the resulting list by that object.
// Use LabelSelectorParam instead for user input.
func (b *Builder) LabelSelector(selector string) *Builder {
    if len(selector) == 0 {
        return b
    }

    b.labelSelector = &selector
    return b
}
```

上面两个函数就是检查是否有selector，如果没有就跳过不设置。如果有，就赋值给了labelSelector。这里我们的例子是没有带selector，所以这里我们跳过不设置。

下面是Flatten()函数

```go
// Flatten will convert any objects with a field named "Items" that is an array of runtime.Object
// compatible types into individual entries and give them their own items. The original object
// is not passed to any visitors.
func (b *Builder) Flatten() *Builder {
    b.flatten = true
    return b
}

```

上面函数将builder的flatten设置为了true。

最后是Do()函数

```go
// Do returns a Result object with a Visitor for the resources identified by the Builder.
// The visitor will respect the error behavior specified by ContinueOnError. Note that stream
// inputs are consumed by the first execution - use Infos() or Object() on the Result to capture a list
// for further iteration.
func (b *Builder) Do() *Result {
    //创建一个Result实例
    r := b.visitorResult()
    r.mapper = b.Mapper()
    if r.err != nil {
        return r
    }
    //如何设置了flatten 创建相应的visitor
    if b.flatten {
        r.visitor = NewFlattenListVisitor(r.visitor, b.objectTyper, b.mapper)
    }
    //下面设置一下help func
    helpers := []VisitorFunc{}
    if b.defaultNamespace {
        helpers = append(helpers, SetNamespace(b.namespace))
    }
    if b.requireNamespace {
        helpers = append(helpers, RequireNamespace(b.namespace))
    }
    helpers = append(helpers, FilterNamespace)
    if b.requireObject {
        helpers = append(helpers, RetrieveLazy)
    }
    // 如果continueOnError设置了，则创建相应的visitor
    if b.continueOnError {
        r.visitor = NewDecoratedVisitor(ContinueOnErrorVisitor{r.visitor}, helpers...)
    } else {
        r.visitor = NewDecoratedVisitor(r.visitor, helpers...)
    }
    return r
}
```

先来看看[b.visitorResult()](https://github.com/kubernetes/kubernetes/blob/release-1.18/staging/src/k8s.io/cli-runtime/pkg/resource/builder.go#L792)函数。

```go
func (b *Builder) visitorResult() *Result {
    //有错误，退出
    if len(b.errs) > 0 {
        return &Result{err: utilerrors.NewAggregate(b.errs)}
    }

    if b.selectAll {
        selector := labels.Everything().String()
        b.labelSelector = &selector
    }

    // 如果paths不空，则返回visitByPaths()结果
    // visit items specified by paths
    if len(b.paths) != 0 {
        return b.visitByPaths()
    }

    // 如果labelSelector不空或者fieldSelector不空，则返回visitBySelector()结果
    // visit selectors
    if b.labelSelector != nil || b.fieldSelector != nil {
        return b.visitBySelector()
    }

    // 如果resourceTuples不空，则返回visitByResource()结果
    // visit items specified by resource and name
    if len(b.resourceTuples) != 0 {
        return b.visitByResource()
    }

    // 如果names不空，则返回visitByName()结果
    // visit items specified by name
    if len(b.names) != 0 {
        return b.visitByName()
    }

    if len(b.resources) != 0 {
        for _, r := range b.resources {
            _, err := b.mappingFor(r)
            if err != nil {
                return &Result{err: err}
            }
        }
        return &Result{err: fmt.Errorf("resource(s) were provided, but no name, label selector, or --all flag specified")}
    }
    return &Result{err: missingResourceError}
}
```

上面这个函数返回了一个Result结构,下面大致看下它有哪些成员。

```go
// Result contains helper methods for dealing with the outcome of a Builder.
type Result struct {
    err     error
    visitor Visitor

    sources            []Visitor
    singleItemImplied  bool
    targetsSingleItems bool

    mapper       *mapper
    ignoreErrors []utilerrors.Matcher

    // populated by a call to Infos
    info []*Info
}

// Visit implements the Visitor interface on the items described in the Builder.
// Note that some visitor sources are not traversable more than once, or may
// return different results.  If you wish to operate on the same set of resources
// multiple times, use the Infos() method.
func (r *Result) Visit(fn VisitorFunc) error {
    if r.err != nil {
        return r.err
    }
    err := r.visitor.Visit(fn)
    return utilerrors.FilterOut(err, r.ignoreErrors...)
}
```

上面我们的例子是通过一个简单的文件方式调用create的。那么我们经过FilenameParam函数过后，paths就有值了。所以在visitorResult()函数中，我们应该返回的是[b.visitByPaths()](https://github.com/kubernetes/kubernetes/blob/release-1.18/staging/src/k8s.io/cli-runtime/pkg/resource/builder.go#L1047)结果。我们下面来看看这个函数具体实现。

```go
func (b *Builder) visitByPaths() *Result {
    //创建一个Result实例
    result := &Result{
        singleItemImplied:  !b.dir && !b.stream && len(b.paths) == 1,
        targetsSingleItems: true,
    }

    if len(b.resources) != 0 {
        return result.withError(fmt.Errorf("when paths, URLs, or stdin is provided as input, you may not specify resource arguments as well"))
    }
    if len(b.names) != 0 {
        return result.withError(fmt.Errorf("name cannot be provided when a path is specified"))
    }
    if len(b.resourceTuples) != 0 {
        return result.withError(fmt.Errorf("resource/name arguments cannot be provided when a path is specified"))
    }

    var visitors Visitor
    //如果continueOnError是true就将paths转换为EagerVisitorList，否则转换为VisitorList
    if b.continueOnError {
        visitors = EagerVisitorList(b.paths)
    } else {
        visitors = VisitorList(b.paths)
    }

    //flatten 设置了，则创建FlattenListVisitor
    if b.flatten {
        visitors = NewFlattenListVisitor(visitors, b.objectTyper, b.mapper)
    }

    //如果latest设置，则创建相应的visitor
    // only items from disk can be refetched
    if b.latest {
        // must set namespace prior to fetching
        if b.defaultNamespace {
            visitors = NewDecoratedVisitor(visitors, SetNamespace(b.namespace))
        }
        visitors = NewDecoratedVisitor(visitors, RetrieveLatest)
    }
    //如果labelSelector不为空，则创建相应的visitor
    if b.labelSelector != nil {
        selector, err := labels.Parse(*b.labelSelector)
        if err != nil {
            return result.withError(fmt.Errorf("the provided selector %q is not valid: %v", *b.labelSelector, err))
        }
        visitors = NewFilteredVisitor(visitors, FilterByLabelSelector(selector))
    }

    //将新创建的visitors赋值给了result.visitor
    result.visitor = visitors
    result.sources = b.paths
    return result
}
```

好，我们来看看我们这种情况下，通过这个函数是嵌套了哪些Visitor。首先我们知道这个continueOnError是true，所有我们会执行到visitors = EagerVisitorList(b.paths)这里。这个语句的意思就是把paths,一个Visitor的slice强制转换成了EagerVisitorList。下面这个flatten也是个true，所以又通过NewFlattenListVisitor(visitors, b.objectTyper, b.mapper)创建了个Visitor， 并且把之前创建的EagerVisitorList传入了。显然我们没有设置latest这个参数，所以不会走入条件里面。下面再是labelSelector，虽然我们有调用LabelSelectorParam(o.Selector)，但是从我们上面举的例子，我们并没有传入selector这个flag。所以也不会满足条件。

好了，到这里我们捋一下，我们现在的Visitor是怎样的结构了。FlattenListVisitor -> EagerVisitorList -> slice(FileVisitor -> StreamVisitor).

我们在回过头来继续分析Do函数里面。首先 flatten 是ture，所以又将r.visitor传入并通过NewFlattenListVisitor创建了FlattenListVisitor返回给了r.visitor。然后continueOnError是true，就又创建了ContinueOnErrorVisitor，并把之前创建的visitor传入。再通过NewDecoratedVisitor 函数创建一个DecoratedVisitor，并传入了新创建的ContinueOnErrorVisitor。

到此。最终的多层Visitor嵌套到此为止。我们来看看现在的Visitor是个怎么样的嵌套关系呢！DecoratedVisitor -> ContinueOnErrorVisitor -> FlattenListVisitor -> FlattenListVisitor -> EagerVisitorList -> slice(FileVisitor -> StreamVisitor).

好了，那当我们在上面RunCreate函数里调用r.Visit()函数时，我们现在来看它是怎么一层一层调用的。下面来看看各个Visitor的实现。我们按照上面我们分析的的顺序一个一个来看。

首先调用的是DecoratedVisitor，

```go
// DecoratedVisitor will invoke the decorators in order prior to invoking the visitor function
// passed to Visit. An error will terminate the visit.
type DecoratedVisitor struct {
    visitor    Visitor
    decorators []VisitorFunc
}

// Visit implements Visitor
func (v DecoratedVisitor) Visit(fn VisitorFunc) error {
    return v.visitor.Visit(func(info *Info, err error) error {
        if err != nil {
            return err
        }
        for i := range v.decorators {
            if err := v.decorators[i](info, nil); err != nil {
                return err
            }
        }
        return fn(info, nil)
    })
}
```

我们可以看到，它是调用了成员visitor的Visit。并将传入的VisitorFunc封装到了一个匿名函数中给成员函数visitor的Visit函数去了。那我们知道这个成员visitor其实是ContinueOnErrorVisitor。那就是将会调用到ContinueOnErrorVisitor的Visit函数。

下面来看看ContinueOnErrorVisitor

```go
// ContinueOnErrorVisitor visits each item and, if an error occurs on
// any individual item, returns an aggregate error after all items
// are visited.
type ContinueOnErrorVisitor struct {
    Visitor
}

// Visit returns nil if no error occurs during traversal, a regular
// error if one occurs, or if multiple errors occur, an aggregate
// error.  If the provided visitor fails on any individual item it
// will not prevent the remaining items from being visited. An error
// returned by the visitor directly may still result in some items
// not being visited.
func (v ContinueOnErrorVisitor) Visit(fn VisitorFunc) error {
    errs := []error{}
    err := v.Visitor.Visit(func(info *Info, err error) error {
        if err != nil {
            errs = append(errs, err)
            return nil
        }
        if err := fn(info, nil); err != nil {
            errs = append(errs, err)
        }
        return nil
    })
    if err != nil {
        errs = append(errs, err)
    }
    if len(errs) == 1 {
        return errs[0]
    }
    return utilerrors.NewAggregate(errs)
}

```

上面同样，ContinueOnErrorVisitor通过调用匿名成员Visitor的Visit函数。我们从嵌套链中看到这个Visitor是FlattenListVisitor。同样，这里也是将Visit函数中传入的VisitorFunc封装到了Visitor的Visit函数中。

下面来看看FlattenListVisitor

```go
// FlattenListVisitor flattens any objects that runtime.ExtractList recognizes as a list
// - has an "Items" public field that is a slice of runtime.Objects or objects satisfying
// that interface - into multiple Infos. Returns nil in the case of no errors.
// When an error is hit on sub items (for instance, if a List contains an object that does
// not have a registered client or resource), returns an aggregate error.
type FlattenListVisitor struct {
    visitor Visitor
    typer   runtime.ObjectTyper
    mapper  *mapper
}

func (v FlattenListVisitor) Visit(fn VisitorFunc) error {
    return v.visitor.Visit(func(info *Info, err error) error {
        if err != nil {
            return err
        }
        if info.Object == nil {
            return fn(info, nil)
        }
        if !meta.IsListType(info.Object) {
            return fn(info, nil)
        }

        items := []runtime.Object{}
        itemsToProcess := []runtime.Object{info.Object}

        for i := 0; i < len(itemsToProcess); i++ {
            currObj := itemsToProcess[i]
            if !meta.IsListType(currObj) {
                items = append(items, currObj)
                continue
            }

            currItems, err := meta.ExtractList(currObj)
            if err != nil {
                return err
            }
            if errs := runtime.DecodeList(currItems, v.mapper.decoder); len(errs) > 0 {
                return utilerrors.NewAggregate(errs)
            }
            itemsToProcess = append(itemsToProcess, currItems...)
        }

        // If we have a GroupVersionKind on the list, prioritize that when asking for info on the objects contained in the list
        var preferredGVKs []schema.GroupVersionKind
        if info.Mapping != nil && !info.Mapping.GroupVersionKind.Empty() {
            preferredGVKs = append(preferredGVKs, info.Mapping.GroupVersionKind)
        }
        errs := []error{}
        for i := range items {
            item, err := v.mapper.infoForObject(items[i], v.typer, preferredGVKs)
            if err != nil {
                errs = append(errs, err)
                continue
            }
            if len(info.ResourceVersion) != 0 {
                item.ResourceVersion = info.ResourceVersion
            }
            if err := fn(item, nil); err != nil {
                errs = append(errs, err)
            }
        }
        return utilerrors.NewAggregate(errs)

    })
}
```

这里也是FlattenListVisitor有一个Visitor的成员。并通过调用成员的Visit函数去执行。同理，这里也是将Visit函数中传入的VisitorFunc封装到了成员visitor的Visit函数中. 从链中，下一个Visitor还是FlattenListVisitor，所以这里是一样的。又嵌套了一层。再下面就是EagerVisitorList了。

我们再看看EagerVisitorList。

```go
type EagerVisitorList []Visitor

// Visit implements Visitor, and gathers errors that occur during processing until
// all sub visitors have been visited.
func (l EagerVisitorList) Visit(fn VisitorFunc) error {
    errs := []error(nil)
    for i := range l {
        if err := l[i].Visit(func(info *Info, err error) error {
            if err != nil {
                errs = append(errs, err)
                return nil
            }
            if err := fn(info, nil); err != nil {
                errs = append(errs, err)
            }
            return nil
        }); err != nil {
            errs = append(errs, err)
        }
    }
    return utilerrors.NewAggregate(errs)
}
```

上面看到EagerVisitorList是一个Visitor slice，它循环的调用slice中的visitor的Visit函数。按照上面分析，这个EagerVisitorList的每个元素是一个FileVisitor。所有它这里就是循环的调用FileVisitor。

下面我们来看看FileVisitor。

```go
// FileVisitor is wrapping around a StreamVisitor, to handle open/close files
type FileVisitor struct {
    Path string
    *StreamVisitor
}

// Visit in a FileVisitor is just taking care of opening/closing files
func (v *FileVisitor) Visit(fn VisitorFunc) error {
    var f *os.File
    if v.Path == constSTDINstr {
        f = os.Stdin
    } else {
        var err error
        f, err = os.Open(v.Path)
        if err != nil {
            return err
        }
        defer f.Close()
    }

    // TODO: Consider adding a flag to force to UTF16, apparently some
    // Windows tools don't write the BOM
    utf16bom := unicode.BOMOverride(unicode.UTF8.NewDecoder())
    v.StreamVisitor.Reader = transform.NewReader(f, utf16bom)

    return v.StreamVisitor.Visit(fn)
}
```

这里看到FileVisitor 有一个匿名指针成员StreamVisitor。并在方法Visit中调用了匿名指针成员StreamVisitor的Visit函数。这里比较直接，没有封装VisitorFunction，而是直接传入了StreamVisitor的Visit函数中。

好了，最后我们来看看StreamVisitor。

```go
// StreamVisitor reads objects from an io.Reader and walks them. A stream visitor can only be
// visited once.
// TODO: depends on objects being in JSON format before being passed to decode - need to implement
// a stream decoder method on runtime.Codec to properly handle this.
type StreamVisitor struct {
    io.Reader
    *mapper

    Source string
    Schema ContentValidator
}

// Visit implements Visitor over a stream. StreamVisitor is able to distinct multiple resources in one stream.
func (v *StreamVisitor) Visit(fn VisitorFunc) error {
    //创建Yaml or Json 的解码器
    d := yaml.NewYAMLOrJSONDecoder(v.Reader, 4096)
    for {
        ext := runtime.RawExtension{}
        //解码
        if err := d.Decode(&ext); err != nil {
            if err == io.EOF {
                return nil
            }
            return fmt.Errorf("error parsing %s: %v", v.Source, err)
        }
        // TODO: This needs to be able to handle object in other encodings and schemas.
        //去空格
        ext.Raw = bytes.TrimSpace(ext.Raw)
        if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
            continue
        }
        //校验schema
        if err := ValidateSchema(ext.Raw, v.Schema); err != nil {
            return fmt.Errorf("error validating %q: %v", v.Source, err)
        }
        //将给定的data转换为info结构
        info, err := v.infoForData(ext.Raw, v.Source)
        if err != nil {
            if fnErr := fn(info, err); fnErr != nil {
                return fnErr
            }
            continue
        }
        if err := fn(info, nil); err != nil {
            return err
        }
    }
}
// InfoForData creates an Info object for the given data. An error is returned
// if any of the decoding or client lookup steps fail. Name and namespace will be
// set into Info if the mapping's MetadataAccessor can retrieve them.
func (m *mapper) infoForData(data []byte, source string) (*Info, error) {
    obj, gvk, err := m.decoder.Decode(data, nil, nil)
    if err != nil {
        return nil, fmt.Errorf("unable to decode %q: %v", source, err)
    }

    name, _ := metadataAccessor.Name(obj)
    namespace, _ := metadataAccessor.Namespace(obj)
    resourceVersion, _ := metadataAccessor.ResourceVersion(obj)

    ret := &Info{
        Source:          source,
        Namespace:       namespace,
        Name:            name,
        ResourceVersion: resourceVersion,

        Object: obj,
    }

    if m.localFn == nil || !m.localFn() {
        restMapper, err := m.restMapperFn()
        if err != nil {
            return nil, err
        }
        mapping, err := restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
        if err != nil {
            return nil, fmt.Errorf("unable to recognize %q: %v", source, err)
        }
        ret.Mapping = mapping

        client, err := m.clientFn(gvk.GroupVersion())
        if err != nil {
            return nil, fmt.Errorf("unable to connect to a server to handle %q: %v", mapping.Resource, err)
        }
        ret.Client = client
    }

    return ret, nil
}
```

绕了一大圈，这里才是真正的读文件的操作，并转换为k8s资源结构。从StreamVisitor的Visit方法中可以看到，通过Json或者Yaml的的格式将内容读到内存，然后解码，在校验schema，再通过infoForData函数转出info类型，最后再调用VisitorFunc函数处理info数据。这里真正的创建函数就是下面这个函数，虽然在传递过程中多少有被其他的Visitor封装了下。

```go
    err = r.Visit(func(info *resource.Info, err error) error {
        if err != nil {
            return err
        }
        if err := util.CreateOrUpdateAnnotation(cmdutil.GetFlagBool(cmd, cmdutil.ApplyAnnotationsFlag), info.Object, scheme.DefaultJSONEncoder()); err != nil {
            return cmdutil.AddSourceToErr("creating", info.Source, err)
        }

        if err := o.Recorder.Record(info.Object); err != nil {
            klog.V(4).Infof("error recording current command: %v", err)
        }

        if o.DryRunStrategy != cmdutil.DryRunClient {
            if o.DryRunStrategy == cmdutil.DryRunServer {
                if err := o.DryRunVerifier.HasSupport(info.Mapping.GroupVersionKind); err != nil {
                    return cmdutil.AddSourceToErr("creating", info.Source, err)
                }
            }
            //创建资源
            obj, err := resource.
                NewHelper(info.Client, info.Mapping).
                DryRun(o.DryRunStrategy == cmdutil.DryRunServer).
                Create(info.Namespace, true, info.Object)
            if err != nil {
                return cmdutil.AddSourceToErr("creating", info.Source, err)
            }
            info.Refresh(obj, true)
        }

        count++

        return o.PrintObj(info.Object)
    })
```

到这里我们可以看到，最后就是调用了Create的方法把相应的资源创建出来了。至此，create的命令就执行成功了。

我们下面再来捋一下VisitorFunc在这些visitor里面的调用和释放顺序。

由于类似于回调的方式，所以最里层的，最先调用。所以调用顺序如下：(StreamVisitor -> FileVisitor) -> EagerVisitorList -> FlattenListVisitor -> FlattenListVisitor -> ContinueOnErrorVisitor -> DecoratedVisitor.

这些VisitorFunc基本上都是一层一层传入的。所以最外层的Func会最先完成。所以释放顺序如下：DecoratedVisitor -> ContinueOnErrorVisitor -> FlattenListVisitor -> FlattenListVisitor -> EagerVisitorList -> (FileVisitor -> StreamVisitor).
