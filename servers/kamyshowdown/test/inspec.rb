describe file('/opt/dayz-server/serverDZ.cfg') do
    its('sha256sum') { should eq 'c399fb6cdccad4147bed53b84d5e042d1e5336c6bcc5364ee5da1bc5de7db128' }
end

describe file('/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/init.c') do
    its('sha256sum') { should eq 'c2ae1eb86ec58fd3dbfc39a728e04f53062cc5167aca87ed05c6da62fb2d904c' }
end

describe file('/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/cfgplayerspawnpoints.xml') do
    its('sha256sum') { should eq '82f6edeec5ac79d52baec4e287dfe8414e510a54f2c78e5f7ee1d41b273e3c13' }
end

describe file('/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/db/globals.xml') do
    its('sha256sum') { should eq '94f32e43e6e1dafd4a5107d7d85356868bab525b2e3eee473679ac40249e1af7' }
end

describe file('/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/db/events.xml') do
    its('sha256sum') { should eq '1a0d5df5e2815c57c92278e7f51fc08c828c902b2e513b3c977cb4db375e7ec61a0d5df5e2815c57c92278e7f51fc08c828c902b2e513b3c977cb4db375e7ec6' }
end

describe file('/opt/dayz-server/mpmissions/dayzOffline.chernarusplus/env/bear_territories.xml') do
    its('sha256sum') { should eq 'b732255e9a1f772003abd1f69fb673d79253add61c8d205da2de06fecc664fe9' }
end