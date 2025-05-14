# blog-aggregator (or gator)

PostgreSQL version 15.11 required for running.\
Golang version go1.23.6 required.\

Commands registered in blog-aggregator:\
`reset` - deletes all user data from users table (cascades down to feed -> follows -> posts)\
`register <username>` - registers a user and logs them in\
`login <username>` - logs in a user that is not the current user\
`users` - lists all users and shows current user logged in\
`addfeed <title> <url>` - adds feed to feeds table and automatically has current user follow\
`feeds` - prints all current feeds and the user of who added it\
`follow <url>` - follows the feed with the url for the current logged in user\
`following` - prints all feeds that the current user is following\
`agg <time between requests>` - sends requests to the feeds to get all RSSFeed item information and adds to posts\
`browse <limit>` - prints x most recent posts for current user where x is the limit provided (defaults to 2)\