# Generic dayz server tests

[
    '/opt/dayz-server/serverDZ.cfg',
    '/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/init.c',
    '/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/cfgplayerspawnpoints.xml',
    '/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/db/globals.xml', 
].each do |bin|
    describe file(bin) do
        its('mode') { should cmp '0755' }
        its('owner') { should eq 'root' }
        its('group') { should eq 'root' }
        its('type') { should cmp 'file' }
    end
end

describe user('dayz') do
    it { should exist }
    its('group') { should eq 'dayz' }
    its('lastlogin') { should eq nil }
end

describe group('dayz') do
    it { should exist }
end

describe systemd_service('dayz-server') do
    it { should be_installed }
end

describe port(2301) do
    it { should be_listening }
    its('protocols') { should include 'tcp' }
    its('addresses') { should_not include '127.0.0.1' }
    its('addresses') { should include '0.0.0.0' }
    its('processes') {should include 'DayZServer'}
end

describe processes('DayZServer') do
    its('users') { should eq ['dayz'] }
end
