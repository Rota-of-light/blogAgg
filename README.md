-Formarlly gator, now known as blogAgg

-You will need both Postgres and Go installed.

-Run this command to install: 
go install github.com/Rota-of-light/blogAgg@latest

-A config file will need to be created in your home directory:
.gatorconfig.json

-Config file can be found in this repo, here is what it should be inside:
{
  "db_url": "postgres://postgres:Your-Weak@localhost:5432/gator?sslmode=disable"
}

-To run, type blogAgg {cmd} {optional arguments}

-List of commands:
    
    -register   Requires a name to be given, use quotes if there is whitespace
    
    -login      Requires a existing username
    
    -users      No optional arguments
    
    -reset      No optional arguments
    
    -addfeed    Requires a title for the site and its URL
    
    -feeds      No optional arguments
    
    -follow     Requires a already saved URL from addfeed
    
    -following  No optional arguments
    
    -unfollow   Requires a URL that current user is following
    
    -agg        Need a given time for each cycle, need number and letter, example 9s, 10m, 1h
    
    -browse     Required that agg was ran or is running, optional limit: positive whole number, else defaults to 2
