# iap-auth-back
This is an example of server that performs user authorization based on IOS IAP receipt file.
It can be used as a convenient boilerplate for the IOS app's backend server.
The server accepts receipt file, validates it via apple server. If there is an active subscription server generates JSON Web Token (JWT).

The example is oriented on IAP auto-renewal subscriptions. 
auth package contains two middleware that you may want to copy and compliment with your business logic.

The main.go contains example of server that use auth middlewares and simple hello api. 
The main_test.go includes requests for the token and simple hello api.
