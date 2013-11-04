%w{curl mercurial bzr}.each do |package_name|
  package package_name
end

bash "install GVM" do
  user        "root"
  cwd         "/root"

  environment Hash["HOME" => "/root"]

  code        <<-SH
curl -s https://raw.github.com/moovweb/gvm/master/binscripts/gvm-installer -o /tmp/gvm-installer
bash /tmp/gvm-installer
rm /tmp/gvm-installer
SH

  not_if      "test -f /root/.gvm/scripts/gvm"
end

cookbook_file "/etc/profile.d/gvm.sh" do
  mode 0755
end
