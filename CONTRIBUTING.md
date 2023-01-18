# Contributing Guide

- [Ways to Contribute](#ways-to-contribute)
- [Find an Issue](#find-an-issue)
- [Ask for Help](#ask-for-help)
- [Pull Request Lifecycle](#pull-request-lifecycle)
- [Development Environment Setup](#development-environment-setup)
- [Sign Your Commits](#sign-your-commits)
  - [DCO](#dco)
- [Pull Request Checklist](#pull-request-checklist)

Welcome! We are glad that you want to contribute to our project!

As you get started, you are in the best position to give us feedback on areas of
our project that we need help with including:

* Problems found during setting up a new developer environment
* Gaps in our Quickstart Guide or documentation
* Bugs in our automation scripts

If anything doesn't make sense, or doesn't work when you run it, please open a
bug report and let us know!

## Ways to Contribute

We welcome many different types of contributions including:

* New features
* Builds, CI/CD
* Bug fixes
* Documentation
* Issue Triage
* Answering questions on GitHub Discussions
* Communications / Social Media / Blog Posts
* Release management

Not everything happens through a GitHub pull request. Please [contact us](https://github.com/topolvm/topolvm/discussions)
and let's discuss how we can work together.

## Find an Issue

We have good first issues for new contributors and help wanted issues suitable
for any contributor. [good first issue](https://github.com/topolvm/topolvm/issues?q=is%3Aopen+is%3Aissue+label%3A%22good+first+issue%22) has extra information to
help you make your first contribution. [help wanted](https://github.com/topolvm/topolvm/issues?q=is%3Aopen+is%3Aissue+label%3A%22help+wanted%22) are issues
suitable for someone who isn't a core maintainer and is good to move onto after
your first pull request.

Sometimes there won’t be any issues with these labels. That’s ok! There is
likely still something for you to work on. If you want to contribute but you
don’t know where to start or can't find a suitable issue, you can ask for an
issue to work on the [Discussions](https://github.com/topolvm/topolvm/discussions).

Once you see an issue that you'd like to work on, please post a comment saying
that you want to work on it. Something like "I want to work on this" is fine.

## Ask for Help

The best way to reach us with a question when contributing is to ask on:

* The original github issue

## Pull Request Lifecycle

If you want to make a non-trivial change, such as a new feature or a bug fix to break exiting behavior,
please discuss it first with maintainers on a issue, discussion or a proposal PR you send.

When your PR becomes ready for review, it isn't a draft and all tests pass, we will start
a review process. Reviewers are selected automatically, so you don't care about them.

If your change is approved by us, we will merge it immediately and it will be
shipped on the next monthly release.

When there has been no activity for 30 days, the stale bot will label it stale.
And if there is no activity for another 7 days, it will be closed.

## Development Environment Setup

Our recommended environment is Ubuntu 20.04. Because following steps modify your system globally,
we suggest preparing a dedicated physical or virtual machine.

1. Download the repository.

    ```console
    git clone git@github.com:topolvm/topolvm.git
    ```

2. Install the required tools.

    ```console
    cd topolvm
    make setup
    ```

3. Make changes you wish.

4. Test your changes.

    ```console
    # for unit test and lint
    make test

    # for end-to-end test
    cd e2e
    make start-lvmd
    make test
    ```

## Sign Your Commits

### DCO
Licensing is important to open source projects. It provides some assurances that
the software will continue to be available based under the terms that the
author(s) desired. We require that contributors sign off on commits submitted to
our project's repositories. The [Developer Certificate of Origin
(DCO)](https://probot.github.io/apps/dco/) is a way to certify that you wrote and
have the right to contribute the code you are submitting to the project.

You sign-off by adding the following to your commit messages. Your sign-off must
match the git user and email associated with the commit.

    This is my commit message

    Signed-off-by: Your Name <your.name@example.com>

Git has a `-s` command line option to do this automatically:

    git commit -s -m 'This is my commit message'

If you forgot to do this and have not yet pushed your changes to the remote
repository, you can amend your commit with the sign-off by running 

    git commit --amend -s

## Pull Request Checklist

When you submit your pull request, or you push new commits to it, our automated
systems will run some checks on your new code. We require that your pull request
passes these checks, but we also have more criteria than just that before we can
accept and merge it. We recommend that you check the following things locally
before you submit your code:

- If your code has breaking changes, please update related documents.
- If you add a new feature, please add unit or end-to-end tests for it.
