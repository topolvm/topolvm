# Vagrant example

If you're not on a Linux machine, we ship a _Vagrantfile_ which sets up a Linux VM using [Vagrant](https://www.vagrantup.com/).
It requires [VirtualBox](https://www.virtualbox.org/) and the [vagrant-disksize](https://github.com/sprotheroe/vagrant-disksize) plugin.
Once Vagrant is setup, add the _vagrant-disksize_ plugin:
```console
$ vagrant plugin install vagrant-disksize
```
and bring your VM up
```console
$ vagrant up
$ vagrant ssh
$ cd /vagrant/example
```
Next, run the example as suggested. However, as Vagrant shares the host directory with the virtual machine, you need to specify where the
your volume will be created. In order to do it, just override the `BACKING_STORE` variable. For example:
```
$ make setup
$ make BACKING_STORE=/tmp run
```
Next, follow the steps previously highlighted. Once you're done with your demonstration environment, logout from your VM and run:
```console
$ vagrant destroy
```
to clean up your environment.
