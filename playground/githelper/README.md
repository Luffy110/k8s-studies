
***This is only an example for how to use cobra lib. because this example has a little bit convenicense only when you want to add reviewers in your commit. if you are using gerrit, i think you have this unpleasent experience which you have always to manually add reviewers in your commit***

## Build code
Using following two commands to get 3pp denpendences and build code.

```shell
go mod vendor
go build .
```

## Usage

```shell
// push commit with a signal reviewer
./githelper push origin -b master -r xxx.gmail.com

//push commit with multiple reviewers in reviewers.txt file.
./githelper push origin -b master --reviewers reviewers.txt

// pull rebase remote code
./githelper pull --rebase

//reset your local repo header
./githelper reset --hard HEAD^ 

```
