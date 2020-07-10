***THIS IS AN EXAMPLE OF THE K8S DYNAMIC ADMISSION WEBHOOK BASED ON CONTROLLER RUNTIME.***

This example is only a testing example, So the base code had implemented that all of opreation of update **deployment** and **scale** are not allowed. Hence you need updat your code if you want other logic.

**You can use following steps to use this example.**

## Build Code
You can use ***build.sh*** script to build code.

## Build image
You can use ***buildimage.sh*** script to build an image.

## Deploy service
You can use Helm install command to instal l deployment..., if you havn't helm ENV, you can manually use **kubectl apply** to install all of these resources.

**NOTE**: If you use **kubectl apply** command to install these k8s resources, you maybe also need manually update **Release.Namespace** to specific namespace you installed, such as **default**.

