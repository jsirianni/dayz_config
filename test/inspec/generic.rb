# Generic dayz server tests

[
    '/opt/dayz-server/serverDZ.cfg',
    '/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/init.c',
    '/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/cfgplayerspawnpoints.xml',
    '/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/db/globals.xml',
    '/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/db/events.xml',  
    '/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/env/bear_territories.xml',
].each do |bin|
    describe file(bin) do
        its('owner') { should eq 'dayz' }
        its('group') { should eq 'dayz' }
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
    its('protocols') { should include 'udp' }
    its('addresses') { should_not include '127.0.0.1' }
    its('addresses') { should include '0.0.0.0' }
end

describe processes('DayZServer') do
    its('users') { should eq ['dayz'] }
end
