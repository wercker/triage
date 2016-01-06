Triage Really Invigorates All Github Experiences
================================================

Triage is a small, opinionated, tool for managing your github issues for an
organization.

Quickstart::

  $ glide in
  $ glide install
  $ go build

  grab yourself a github api key

  $ ./triage ui --api-token=<your api key>


What you'll see is a list of all the open issues (currently sorted by modified),you can scroll through them and note the menu options. If, for example, you hit
"m" then scroll through them you can hit "1" to mark the ones for this milestone, "2" for the next and so on

Surely full of bugs, most of them might not kill you. Pretty much panics on
anything that goes wrong with hopes that you'll figure out what's going on
and file a patch ;)


Some Useful Setup Helpers
-------------------------

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


--------------
Listing Labels
--------------

Pick a project with labels you like and put these under the `types`
or `priorities` sections of your config as is appropriate and in the order
you want to sort them in::

  # show the labels for a project
  $ triage show-labels owner/repo


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
so don't mess around with milestones manually
TODO(termie): warn if nearer date noticed when creating

::

  # create a new milestone in a project (due next week monday, more or less)
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



Github Search will only give you up to 1000 results, so if you've got a ton
more than that you're going to want to make specific triage calls.


Everything that follows is a lie.

::

  Config
  ------

  Orgmode
    org: wercker # organization we'll be acting for
    milestones:  # assigned a number hotkey based on next due date
      - name: foo
        date: xxx
        desc: asdsadas

    labels:
      - name: bug
        color: red
        hotkey: b
      - name: enhancement
        color: blue
        hotkey: e

    projects: # sync from interface?
      - name
      - name
      - name



  Singlemode
    project: name
    milestones:  # assigned a number hotkey based on next due date
      - Name: foo
      - Date: xxx
      - Description: asdsadas
    labels:
      - name: bug
        color: red
        hotkey: b
      - name: enhancement
        color: blue
        hotkey: e

  Windows
  -------

  Repo:
    Select repos to filter on
    Refresh repos
    (Load repos from local cache)

  Milestones:
    List milestones from selected projects
      Compact:
        "milestone title -> projectname, projectname, projectname"
        "milestone title -> projectname, projectname"
      Expanded:
        milestone title
          project
          * project
          * project
        milestone title

    Set milestones across projects
    Delete milestones

  Issue List:
    List of search issues
    List of issues for a project
    Quick assign labels to issues via hotkey
    Filter issues by typing
    Quick-view issue
    Hotkey + Filter Assign
    Hotkey + Filter Milestone

  Expanded Issue:
    Issue Text
    Comments
    Reply
    Hotkeys for labeling
    Hotkey + Filter Assign
    Hotkey + Filter Milestone


  UI Concepts
  -----------

  Header
    - Tabs
  List + Cursor
  Expandable Sublist
  Scrollable List
  Scrollable Text
  Hotkey
  Pop-up menu with filter
  Filtering
  Switch to Text Editor


  Commands
  --------

  - sync milestones
    - gather all milestones
    - delete milestones
    - set milestones for all selected projects


