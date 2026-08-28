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
Select the source calendars that RSVP can read.

Select **Synchronize** for each source calendar.
Later synchronizations use the saved sync cursor.
Calendar visibility and display order remain RSVP values.

## Use Public Invitations

Open an RSVP QR page from the event RSVP controls.
Print the QR code or share its public response link.

An invitee can open the public link without organizer authentication.
The invitee can select a response and an extra guest count.
