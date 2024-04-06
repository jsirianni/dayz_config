describe file('/opt/dayz-server/serverDZ.cfg') do
    its('sha256sum') { should eq 'c399fb6cdccad4147bed53b84d5e042d1e5336c6bcc5364ee5da1bc5de7db128' }
end

describe file('/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/init.c') do
    its('sha256sum') { should eq 'df1b6abf50adcf4be009c1bce7c9b032df81d2916426ec714f4a578e69c41896' }
end

describe file('/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/cfgplayerspawnpoints.xml') do
    its('sha256sum') { should eq '82f6edeec5ac79d52baec4e287dfe8414e510a54f2c78e5f7ee1d41b273e3c13' }
end

describe file('/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/db/globals.xml') do
    its('sha256sum') { should eq 'a2fbeed33718d975c328af5815419c815ad1cb34fb813302a0668f49ba6e2c82' }
end
