# Time Horizon User Guide

The horizon view is the main organizer interface.
It shows calendars, lanes, event markers, derived markers, and attention probes on one time axis.

## Read The Horizon

Each calendar is a visibility group.
Each lane is one independent event, one event series, one dependency chain, or one open subject.

An open lane has a continuation arrow.
A finite lane has an end cap.
Calendar names and lane shapes supply meaning without color.

Select a marker to show its details.
Use the event and RSVP links when the selected marker is an event.

## Control The View

Use the arrow controls to pan through time.
Use the `D`, `W`, `M`, and `Y` controls to select the time scale.
The day scale shows one local day.
The week scale shows seven local days.
The month scale shows one calendar month.
The year scale shows one calendar year.
RSVP stores the selected scale as a browser preference.
Each later Horizon load uses the stored scale.
Each arrow moves the window by one selected scale.
Use a calendar checkbox to show or hide its lanes.
The time window row is directly above the timeline.
The row contains the range, timezone, pan controls, and scale controls.

Open **Settings**, and then open **Help** to read the keyboard operations.
Use these operations when the horizon view has focus:

- Press `h` or `l` to pan.
- Press `d`, `w`, `m`, or `y` to select the time scale.
- Press a number key to change calendar visibility.
- Press `j` or `k` to select a marker.

## Manage Calendars And Lanes

Open **Manage horizon** to create, change, reorder, or delete calendars and lanes.
Calendar selection controls visibility membership.
It does not put independent events on one lane.

Create an open lane when its end is not known.
Resolve an open lane when RSVP must stop its future attention probes.

## Change Account Settings

Open **Settings** and select **Account**.
Find the **Timezone** account setting.
Enter or select one IANA timezone name.
Select **Use browser timezone** to copy the current browser timezone.
Select **Save timezone** to store the value.

The next default Horizon window uses the changed timezone.
The timezones of stored events do not change.
RSVP keeps the current organizer timezone when the new value is invalid.

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
Select **Create connection** after RSVP verifies consent.
RSVP creates the connection and starts a calendar import task.
The page opens the Integrations rubric without waiting for the import.
The rubric shows `Queued`, `Running`, or `Complete` for the task.
If an attempt fails, the rubric shows the scheduled retry or the required attention.
For a new organizer, RSVP uses the browser timezone when it creates the connection.
If the browser timezone is absent or invalid, RSVP uses `UTC`.
RSVP imports each Google calendar that has event read access.
This import includes hidden calendars such as Birthdays, Holidays, and Family.
The Google Contacts birthday source does not create a separate visible calendar.
RSVP puts normalized birthday events from all source calendars in one `Birthdays` calendar.
RSVP translates Google API values into useful calendar names.
RSVP also recognizes the complete title words `birthday`, `birthdays`, and `bday`.
This rule corrects birthday events that Google labels as ordinary events.
An unknown Google event type stays in its Google calendar and does not create a new group.
Anniversaries and other non-birthday special dates stay in their source calendar.
The first complete import replaces prior local calendar groupings.

Each Google calendar becomes one RSVP calendar.
Independent events use separate lanes in that calendar.
Occurrences in one event series use one shared lane.

Google selection sets the initial RSVP calendar visibility.
RSVP uses the provider calendar meaning for its name.
RSVP gives each visible calendar a distinct presentation color.
The color stays the same when you hide, show, or add a calendar.
You can show up to eight calendars at the same time.
Hide one calendar before you show a ninth calendar.
You can then change the RSVP visibility, color, and display order.
Background synchronization keeps these RSVP values.
It imports new Google calendars and applies Google calendar name changes automatically.
Synchronization can move an event when its normalized birthday meaning changes.

## Use Public Invitations

Open an RSVP QR page from the event RSVP controls.
Print the QR code or share its public response link.

An invitee can open the public link without organizer authentication.
The invitee can select a response and an extra guest count.
