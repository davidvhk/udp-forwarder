%global debug_package %{nil}

Name:           udp-forwarder
Version:        %{?_version}%{!?_version:1.0.0}
Release:        %{?_release}%{!?_release:1}%{?dist}
Summary:        A UDP traffic forwarder
License:        MIT
URL:            https://github.com/davidvhk/udp-forwarder

%description
This project is a UDP traffic forwarder that listens for incoming UDP packets and forwards them to specified destinations based on a configuration file.

%prep
# Nothing to do

%build
# Nothing to do

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}/usr/local/bin
mkdir -p %{buildroot}/etc/udp-forwarder
mkdir -p %{buildroot}/usr/lib/systemd/system

install -p -m 755 %{_sourcedir}/bin/udp-forwarder %{buildroot}/usr/local/bin/udp-forwarder
install -p -m 644 %{_sourcedir}/config/config.yaml %{buildroot}/etc/udp-forwarder/config.yaml
install -p -m 644 %{_sourcedir}/udp-forwarder.service %{buildroot}/usr/lib/systemd/system/udp-forwarder.service

%files
/usr/local/bin/udp-forwarder
/usr/lib/systemd/system/udp-forwarder.service
%config(noreplace) /etc/udp-forwarder/config.yaml

%changelog
* Wed Aug 19 2026 David Vanhoucke <vanhouckedavid@gmail.com> - 1.0.0-1
- Add RPM and DEB release workflow and Go linter
