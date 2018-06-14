Triage Really Invigorates All Github Experiences
================================================

Triage is a small, opinionated, tool for managing your github issues for an
organization, multiple projects, or just yourself.

It is based on processes written about in these two links:

http://www.ianbicking.org/blog/2014/03/use-github-issues-to-organize-a-project.html
http://www.stateofcode.com/2013/06/using-github-issues-effectively/

Here are some screenshots:

 .. image:: https://cloud.githubusercontent.com/assets/56459/12189793/2b272dfe-b576-11e5-9d65-f192100a1627.png

 .. image:: https://cloud.githubusercontent.com/assets/56459/12189794/2cdd46d8-b576-11e5-86d9-60901ef556f4.png


Before You Start
----------------

Grab yourself a personal access token from Github:

  https://github.com/settings/tokens

Add it to your env as GITHUB_TOKEN


Quickstart: How To Use
----------------------

Go grab a release from: https://github.com/wercker/triage/releases

PRO TIP: Instead of typing `--api-token=<your api token>`, export `GITHUB_TOKEN`

::

  $ triage --api-token=<your api token> ui repo:some/repo

What you'll see is a list of all the open issues (sorted by number until you
start prioritizing things).

You can see everything for an org (might take a while) by using::

  $ triage --api-token=<your api token> ui --org your_org

Or just the stuff assigned to you (only really there so that first-run works)::

  $ triage --api-token=<your api token> ui


Hit "?" for help, it's super cool.

You can scroll through them with up/down, esc and left will back you out of
things.

When the cursor is over the filter, you can quick filter by typing stuff.

When the cursor is over the sort, you can type the name of a column to sort.

When the cursor is over an issue there are some menu options showing hotkeys.
If, for example, you hit "p" then scroll through them you can hit "1" to mark
the current issue with priority Blocker, "2" for Critical and so on.

Ctrl-C exits, as do typing ":q" or ":wq" and hitting enter.

You can put config information in `triage.yml`, and eventually TODO(termie) in
something like .triage/config

(If anybody wants to make a screenshare of using this to triage issues that'd
be cool)

-----------------
IDX Sorting Order
-----------------

In ascending order, it will show:

 1. Anything with priority 1 (defaults to "blocker")
 2. Items sorted by Milestone > Priority > Type
 3. In the event of a tie, lowest issue number


-----------------
Filtering Helpers
-----------------

There are a bunch of things being searched for, try `p2` to see all your
priority 2 issues, `m1 p2 t3 la` for all your milestone 1, priority 2, type 3 issues that have an "la" somewhere in the title. The issue number and repo are also in there.




Getting Started
---------------

Triage ships with pretty alright defaults, but if you want it to conform to
stuff you already have set up, there are some tools to output existing info
from your projects so you can copy-paste your way to config victory.

----------------
Listing Projects
----------------

Put these under the `projects` section of your config and delete the ones
you don't want::

  # show all your projects
  $ triage show-projects

  # show all the projects for an org you have access to
  $ triage show-projects some_org

There may be some github limit on number of projects that can be part of your
search query, if so see "So, You Have A Way Too Many Issues" below

--------------
Listing Labels
--------------

Pick a project with labels you like and put these under the `types`
or `priorities` sections of your config as is appropriate and in the order
you want to sort them in::

  # show the labels for a project
  $ triage show-labels owner/repo


In general the labels will create themselves as you use them for priority and
type.

------------------
Initial Milestones
------------------

Create the Next and Someday milestones across all projects, and make your first
Current milestone::

  $ triage set-milestones all
  $ triage create-milestone all

Uses a predictable scheme for randomly chosen milestone titles, so adding new
projects to the current week should Just Work(tm) if you aren't doing anything
weird already.


How Labels Work
---------------

Triage believes in two label dimensions: Priority and Type.

We don't really care a heck of a lot about what they're called (and we've got
some defaults), you just need to define some in your config if you want to
customize them.

From there, you can setup the labels on your projects using Triage::

  # for an individual project
  $ triage set-labels owner/repo

  # for all projects you've defined in your config
  $ triage set-labels all


How Milestones Work Cross-Project
---------------------------------

Triage believes in 3 conceptual milestones, so you do, too. Congrats, you're
well on your way to a happier life.

They are: Current, Next, Someday

Next and Someday have no due date and you're going to define names for them
that all your projects are going to share (defaults: Next, Someday) and we've
got some tools to help you set up those milestones in new projects. We'll look
those up when we load to get the IDs for them in all the projects we're
watching.

For the Current milestone, you've got two options:

Set up your milestones yourself, when we load we'll associate whichever
milestone has *the nearest due date after now* as "Current"

::

  # show the milestones Triage recognized
  $ triage show-milestones


Or, have Triage make a new milestone in each of your projects. If there is
a milestone with a due date sooner than that, that'll be detected instead,
so don't mess around with milestones manually.
TODO(termie): warn if nearer date noticed when creating

You'll want to create a new milestone at the beginning of each week, it'll be
due the next monday.

::

  # create a new milestone in a project due the first monday after now + 5 days
  # with a fancy name picked based on the year and week,
  $ triage create-milestone owner/repo
  # or use a due date and title
  $ triage create-milestone --due 2016-01-22 --title "I named myself" owner/repo

  # or the same for all projects
  $ triage create-milestone all

  # set the next and someday milestones for an individual project
  $ triage set-milestones owner/repo

  # set the next and someday milestones for all projects in your config
  $ triage set-milestones all

Anything that is not in either of those three detected milestones is considered
Untriaged and will not be considered to have a milestone (and be sorted
accordingly).

If you hate all of that, I can probably add a config option to turn off
any sort of mention of milestones and you can go be sad in your own little
world.


So, You Have A Way Too Many Issues
----------------------------------

Well, for the most part Triage doesn't really care a whole lot which project
it is looking at as long as the setup of the project matches your expected
config. Towards that end, you can pretty much put any search query you want
in as the starting point for it::

  # "is:open is:issue" is currently implied
  $ triage ui "repo:owner/repo searchstring"


Github Search will start getting slow with lots of results, so if you've got a
ton you're going to want to make specific triage calls.


An Example Config
-----------------

Currently, triage only looks for `triage.yml` in the current working directory.

Also happen to show the defaults (besides the `projects` section) that you'll
get if you just run with it::

  triage.yml
    next-milestone: Next
    someday-milestone: Someday

    projects:
      - wercker/foo
      - wercker/bar

    types:
      - name: bug
        color: f7c6c7
      - name: task
        color: fef2c0
      - name: enhancement
        color: bfe5bf
      - name: question
        color: c7def8

    priorities:
      - name: blocker
        color: e11d21
      - name: critical
        color: eb6420
      - name: normal
        color: fbca04
      - name: low
        color: "009800"


How To Build
------------

PRO TIP: https://github.com/wercker/triage/releases

N.B. I'm using a really old glide for various purposes, if all else fails the
glide.yaml has a list of the packages you need.

Manually::

  $ glide in
  $ glide install
  $ go build


Caveat Emptor
-------------

::

  THIS...IS...ALLLLPPPHAAAA!
  .. O
  .. /I_
  .. /
  [][][][][] ... >-/-O....... [][][][]
  [][][][][] ................ [][][][]
  [][][][][] ................ [][][][]


Surely full of bugs, most of them might not kill you. Pretty much panics on
anything that goes wrong with hopes that you'll figure out what's going on
and file a patch ;)

Some known issues:

 - milestone actions silently fail if you don't have the milestone system setup
   (see "Initial Milestones" below).
 - if, for example, a repo can't be found you'll get a panic.
 - you can't scroll through body text, it's just there to remind you of the
   issue (follow the link for more).
 - despite running a company dedicated to build and testing, I did not
   write tests for this, termbox + testing = my brain a splode.
