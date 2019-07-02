# Contributing
OTE Stack is Apache 2.0 licensed and welcome to contribute code through github. When contributing to this repository, please first discuss the change you wish to make via issue, email, or any other method with the owners of this repository before making a change.


## Getting started
* Fork the repository on GitHub
* Read the [README.md](./README.md) for build instructions


## Contribution flow
OTE-Stack use this [Git branching model](https://nvie.com/posts/a-successful-git-branching-model/). The following steps guide usual contributions.

1. Fork

   To make a fork, please refer to Github page and click on the ["Fork"](https://help.github.com/articles/fork-a-repo/) button.

2. Prepare for the development environment for linux

   ```bash
   go get github.com/baidu/ote-stack # get OTE Stack official repository
   cd $GOPATH/src/github.com/baidu/ote-stack # step into OTE Stack
   git checkout master  # verify master branch
   git remote add fork https://github.com/<your_github_account>/ote-stack  # specify remote repository
   ```

3. Push changes to your forked repository

   ```bash
   git status   # view current code change status
   git add .    # add all local changes
   git commit -m "modify description"  # commit changes with comment
   git push fork # push code changes to remote repository which specifies your forked repository
   ```

4. Create pull request

   To create a pull request, please follow [these steps](https://help.github.com/articles/creating-a-pull-request/). Once the OTE-Stack repository reviewer approves and merges your pull request, you will see the code contributed by you in the ote-stack official repository.

## CodeStyle
The coding style suggested by the Golang community is used in OTE-Stack. See the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) for details.
Please follow this style to make OTE-Stack easy to review, maintain and develop.

## Merge Rule
* Please run command `govendor fmt +local` before push changes, more details refer to [govendor](https://github.com/kardianos/govendor)
* Must run command `make test` before push changes(unit test should be contained), and make sure all unit test and data race test passed
* Only the passed(unit test and data race test) code can be allowed to submit to OTE-Stack official repository
* At least one reviewer approved code can be merged into OTE-Stack official repository

