# vim: set ft=ruby

Vagrant.configure("2") do |config|
  config.vm.hostname = "garden"

  config.vm.box = "precise64"
  config.vm.box_url = "http://files.vagrantup.com/precise64.box"

  config.omnibus.chef_version = :latest

  config.vm.synced_folder ENV["GOPATH"], "/workspace"
  config.vm.synced_folder File.expand_path("~/workspace"), "/real-workspace"

  config.vm.provider :virtualbox do |v, override|
    v.customize ["modifyvm", :id, "--memory", 3*1024]
    v.customize ["modifyvm", :id, "--cpus", 4]
  end

  config.vm.provider :vmware_fusion do |v, override|
    override.vm.box_url = "http://files.vagrantup.com/precise64_vmware.box"
    v.vmx["numvcpus"] = "4"
    v.vmx["memsize"] = 3 * 1024
  end

  config.vm.provision :chef_solo do |chef|
    chef.cookbooks_path = ["cookbooks", "site-cookbooks"]

    chef.add_recipe "garden::apt-update"
    chef.add_recipe "build-essential::default"
    chef.add_recipe "chef-golang"
    chef.add_recipe "garden::warden"

    chef.json = {
      go: {
        version: "1.1.2",
      },
    }
  end
end
