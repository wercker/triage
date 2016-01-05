Triage Really Invigorates All Github Experiences
================================================

Triage is a small, opinionated, tool for managing your github issues for an
organization.

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


