# Host time checks without NTP

The `time` collector reports the system's original `ntp_active` and
`ntp_synchronized` flags. On Linux it also looks for the kernel's
`KVM virtual PTP`, `hyperv` (Hyper-V), or `ptp_vmw` (VMware) device and measures the system clock against the hypervisor's
wall clock. This comparison does not need network access.

The new `clock == synchronized` analyzer condition accepts either the original
synchronized flag or a valid hypervisor measurement within one second.
`clock == unsynchronized` is its inverse. Existing `ntp` and `timezone`
conditions keep their original behavior; a host-time match does not mean NTP
is synchronized.

For example, a host `time` analyzer can use:

```yaml
- time:
    checkName: System Clock
    outcomes:
      - pass:
          when: 'clock == synchronized'
          message: System clock is synchronized or agrees with the hypervisor clock
      - fail:
          message: System clock synchronization could not be verified
```

The collector adds an optional `hypervisor_clock` object to
`host-collectors/system/time.json`, containing `source`, `offset_nanoseconds`
(system time minus reference time), and `read_duration_nanoseconds`.
`hypervisor_clock_error` records a failed attempt to read a recognized device.
The original NTP fields are retained, so a report can show NTP unsynchronized
while the clock check passes using the measured hypervisor reference.

Only these kernel-identified hypervisor clocks are accepted. Ordinary PTP devices,
including unsynchronized NIC clocks, are not sufficient. A missing or failed
reference cannot turn an unsynchronized result into a pass. Measurements taking
more than 100 ms are rejected; accepted measurements include half the sampling
interval as uncertainty in the one-second bound. Existing NTP-synchronized
results retain their previous behavior.

This relies on the hypervisor having correct time. It does not establish UTC
accuracy if the hypervisor is wrong, or support every hypervisor. Hosts without
this reference still need another supported synchronization source. No
per-check bypass or warning-only mode is added.

The collector only reads an already exposed clock. It does not load kernel
modules, enable time services, change system time, or configure the host. A
guest without an exposed supported PTP device does not gain a fallback.

Preflight and support-bundle must both contain this implementation. An older
analyzer reading the additional fields will ignore them and retain its old NTP
flag behavior. EC v2/v3 must distribute the updated binaries and opt into the
new `clock` condition in host preflights and support-bundle analyzers. Do not
ship specs using this condition with older Troubleshoot binaries.
