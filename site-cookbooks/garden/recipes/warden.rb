ROOT_FS_URL = "http://cfstacks.s3.amazonaws.com/lucid64.dev.tgz"

%w{
  git
  curl
  debootstrap
  iptables
  ruby1.9.3
}.each do |package_name|
  package package_name
end

if ["debian", "ubuntu"].include?(node["platform"])
  if node["kernel"]["release"].end_with? "virtual"
    package "linux-image-extra" do
      package_name "linux-image-extra-#{node['kernel']['release']}"
      action :install
    end
  end
end

package "quota" do
  action :install
end

package "apparmor" do
  action :remove
end

execute "remove all remnants of apparmor" do
  command "sudo dpkg --purge apparmor"
end

directory "/opt/warden" do
  owner "vagrant"
  mode 0755
  action :create
end

%w(rootfs containers stemcells).each do |dir|
  directory "/opt/warden/#{dir}" do
    owner "vagrant"
    mode 0755
    action :create
  end
end

execute "Install RootFS" do
  cwd "/opt/warden/rootfs"

  command "curl -s #{ROOT_FS_URL} | tar zxf -"
  action :run

  not_if "test -d /opt/warden/rootfs/usr"
end

bash "Setup Warden" do
  cwd "/vagrant/warden/warden"

  code <<-SH
set -e

(cd src/ && make clean all)

cp src/wsh/wshd root/linux/skeleton/bin
cp src/wsh/wsh root/linux/skeleton/bin
cp src/oom/oom root/linux/skeleton/bin

cp src/iomux/iomux-spawn root/linux/skeleton/bin
cp src/iomux/iomux-spawn root/insecure/skeleton/bin

cp src/iomux/iomux-link root/linux/skeleton/bin
cp src/iomux/iomux-link root/insecure/skeleton/bin
SH

  action :run
end
