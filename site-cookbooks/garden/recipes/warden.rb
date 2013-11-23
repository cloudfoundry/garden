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
  mode 0755
  action :create
end

%w(rootfs containers stemcells).each do |dir|
  directory "/opt/warden/#{dir}" do
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

execute "build root directory" do
  cwd "/vagrant"

  command "make"
  action :run
end
