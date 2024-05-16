describe file('/opt/dayz-server/serverDZ.cfg') do
    its('sha256sum') { should eq 'c399fb6cdccad4147bed53b84d5e042d1e5336c6bcc5364ee5da1bc5de7db128' }
end

describe file('/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/db/globals.xml') do
    its('sha256sum') { should eq '94f32e43e6e1dafd4a5107d7d85356868bab525b2e3eee473679ac40249e1af7' }
end
