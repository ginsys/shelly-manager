Fixed

- Bulk import and both drift checks no longer fail after 10 seconds. They wait as long as the server does for the devices, so a result that takes 12 seconds is shown as a result. A request that gets no answer at all now shows a timeout in the dialog, naming the operation and the number of devices, instead of "Contact support". Fixes ginsys/shelly-manager#752.
