# envy
A kubernetes controlled envoy

Envy will ask kubernetes about services and endpoints with the ```schwarzm/envy``` label.
If there are some it will configure the locally spawned aggregated discovery service to
Setup TCP listeners and clusters accordingly.

The IP used is given as the value of the ```schwarzm/envy``` label.


