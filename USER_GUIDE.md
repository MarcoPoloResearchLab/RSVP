# Time Horizon User Guide

The horizon view is the main organizer interface.
It shows calendars, lanes, event markers, derived markers, and attention probes on one time axis.

## Read The Horizon

Each calendar is a visibility group.
Each lane is one independent event, one event series, one dependency chain, or one open subject.

An open lane has a continuation arrow.
A finite lane has an end cap.
Calendar symbols, text, and lane shapes supply meaning without color.

Select a marker to show its details.
Use the event and RSVP links when the selected marker is an event.

## Control The View

Use the arrow controls to pan through time.
Use the plus and minus controls to change the time scale.
Use a calendar checkbox to show or hide its lanes.

Use these keyboard operations when the horizon view has focus:

- Press `h` or `l` to pan.
- Press `-` or `=` to change the time scale.
- Press a number key to change calendar visibility.
- Press `j` or `k` to select a marker.

## Manage Calendars And Lanes

Open **Manage horizon** to create, change, reorder, or delete calendars and lanes.
Calendar selection controls visibility membership.
It does not put independent events on one lane.

Create an open lane when its end is not known.
Resolve an open lane when RSVP must stop its future attention probes.

## Use Attention

Add an attention policy to an active open lane.
Set the review interval and the next probe time.
Set an escalation interval when an overdue probe must become missed.

Complete a pending probe from its lane controls.
RSVP then creates the next probe from the completion time and review interval.

## Create A Quick Draft

Open **Quick add**.
Select **Dated event** or **Open lane**.
Supply the calendar, title, time values, and optional attention values.

Select an anchor event only for a dependent event.
An independent event gets a new lane.
A dependent event uses its anchor event lane.

Review the proposed calendar, lane, marker, and attention values.
Correct each value that is not correct.
Select **Confirm draft** to create the temporal resources.

Select **Cancel draft** to cancel the proposal.
Cancellation does not change temporal resources.

## Use Natural-Language Input

Enter one temporal request in the natural-language draft form.
Select the calendar for the proposed lane.
Select **Parse into draft**.

Review every inferred value and each relative marker rule.
Supply each value in the missing field list.
Save the proposal before confirmation.

RSVP does not store the original input text.
The parser result stays a draft until explicit confirmation.

## Connect Google Calendar

Open **Manage horizon**.
Select **Connect Google Calendar** and complete the separate consent flow.
RSVP imports each Google calendar that has event read access.
This import includes hidden calendars such as Birthdays, Holidays, and Family.
RSVP also imports birthday events from the primary Google calendar into `Birthdays`.
RSVP translates Google API values into useful calendar names.
RSVP also recognizes the complete title words `birthday`, `birthdays`, and `bday`.
This rule corrects birthday events that Google labels as ordinary events.
An unknown Google event type stays in its Google calendar and does not create a new group.
The first complete import replaces prior local calendar groupings.

Each Google calendar becomes one RSVP calendar.
Independent events use separate lanes in that calendar.
Occurrences in one event series use one shared lane.

Google selection sets the initial RSVP calendar visibility.
RSVP uses the calendar meaning for its name and symbol.
RSVP gives each visible calendar a distinct presentation color.
You can then change the RSVP visibility, symbol, color, and display order.
Background synchronization keeps these RSVP values.
It imports new Google calendars and applies Google calendar name changes automatically.

## Use Public Invitations

Open an RSVP QR page from the event RSVP controls.
Print the QR code or share its public response link.

An invitee can open the public link without organizer authentication.
The invitee can select a response and an extra guest count.
