// @ts-check

const {test, expect} = require('@playwright/test');

test.beforeEach(async ({page}) => {
    await page.goto('/browser-login/');
    await expect(page).toHaveURL(/\/horizon\/$/);
    await expect(page.locator('[data-horizon-view]')).toBeVisible();
    await page.goto('/');
    await expect(page).toHaveURL(/\/horizon\/$/);
});

async function openSettings(page, rubric = 'Calendars & lanes') {
    const dialog = page.locator('[data-settings-dialog]');
    if (!await dialog.evaluate((element) => element.open)) {
        const userMenu = page.locator('[data-user-menu]');
        if (!await userMenu.evaluate((element) => element.open)) {
            await userMenu.locator(':scope > summary').click();
        }
        await userMenu.getByRole('button', {name: 'Settings', exact: true}).click();
    }
    const rubricTab = dialog.getByRole('tab', {name: rubric, exact: true});
    if (await rubricTab.getAttribute('aria-selected') !== 'true') {
        await rubricTab.click();
    }
}

async function closeSettings(page) {
    const dialog = page.locator('[data-settings-dialog]');
    await page.waitForLoadState('domcontentloaded');
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', {name: 'Close settings'}).click();
}

async function readHorizonLocalDates(page) {
    const timezone = (await page.locator('.horizon-window span').last().textContent())?.trim();
    const dateTimes = await page.locator('.horizon-window time').evaluateAll((elements) => elements.map((element) => element.getAttribute('datetime')));
    if (!timezone || dateTimes.length !== 2 || dateTimes.some((dateTime) => !dateTime)) {
        throw new Error('The Horizon window date contract is incomplete.');
    }
    return page.evaluate(({values, zone}) => {
        const formatter = new Intl.DateTimeFormat('en-CA', {timeZone: zone, year: 'numeric', month: '2-digit', day: '2-digit'});
        return values.map((value) => {
            const parts = Object.fromEntries(formatter.formatToParts(new Date(value)).map((part) => [part.type, part.value]));
            return `${parts.year}-${parts.month}-${parts.day}`;
        });
    }, {values: dateTimes, zone: timezone});
}

function addHorizonScale(date, scale, direction = 1) {
    const result = new Date(`${date}T00:00:00Z`);
    if (scale === 'day') {
        result.setUTCDate(result.getUTCDate() + direction);
    } else if (scale === 'week') {
        result.setUTCDate(result.getUTCDate() + 7 * direction);
    } else if (scale === 'month') {
        result.setUTCMonth(result.getUTCMonth() + direction);
    } else if (scale === 'year') {
        result.setUTCFullYear(result.getUTCFullYear() + direction);
    } else {
        throw new Error(`Invalid Horizon scale ${scale}.`);
    }
    return result.toISOString().slice(0, 10);
}

test('sets up Horizon after the first authentication', async ({context, page}) => {
    await context.clearCookies();
    await page.goto('/browser-new-login/');
    await expect(page).toHaveURL(/\/horizon\/$/);
    await expect(page.locator('[data-horizon-setup]')).toBeVisible();
    await expect(page.getByRole('textbox', {name: 'Calendar name'})).toHaveValue('Personal');
    await expect(page.getByText('Your time, in motion')).toHaveCount(0);
    await expect(page.getByRole('textbox', {name: 'Symbol'})).toHaveCount(0);
    await page.getByRole('button', {name: 'Start Horizon'}).click();

    await expect(page.locator('[data-horizon-view]')).toBeVisible();
    await expect(page.locator('[data-calendar-toggle]')).toHaveCount(1);
    await expect(page.locator('.horizon-calendar-toggle')).toContainText('Personal');
    await expect(page.locator('.horizon-window')).toContainText('America/Los_Angeles');
    await expect(page.getByText('Your time, in motion')).toHaveCount(0);
});

test('renders the lane and marker contract', async ({page}) => {
    const windowRow = page.locator('[data-horizon-window-row]');
    const windowText = windowRow.locator('.horizon-window');
    const viewControls = windowRow.locator('.horizon-view-controls');
    const quickAdd = page.locator('[data-quick-add]');
    const viewport = page.locator('[data-horizon-viewport]');
    await expect(windowText).toContainText('America/Los_Angeles');
    await expect(viewControls.getByRole('button')).toHaveCount(6);
    await expect(viewControls.getByRole('button')).toHaveText(['←', 'D', 'W', 'M', 'Y', '→']);
    await expect(viewControls.getByRole('button', {name: 'Month scale'})).toHaveAttribute('aria-pressed', 'true');
    const windowBox = await windowText.boundingBox();
    const controlsBox = await viewControls.boundingBox();
    const quickAddBox = await quickAdd.boundingBox();
    const windowRowBox = await windowRow.boundingBox();
    const viewportBox = await viewport.boundingBox();
    expect(windowBox).not.toBeNull();
    expect(controlsBox).not.toBeNull();
    expect(quickAddBox).not.toBeNull();
    expect(windowRowBox).not.toBeNull();
    expect(viewportBox).not.toBeNull();
    expect(Math.abs((windowBox.y + windowBox.height / 2) - (controlsBox.y + controlsBox.height / 2))).toBeLessThanOrEqual(2);
    expect(quickAddBox.y + quickAddBox.height).toBeLessThanOrEqual(windowRowBox.y + 1);
    expect(windowRowBox.y + windowRowBox.height).toBeLessThanOrEqual(viewportBox.y + 1);
    expect(await page.locator('[data-quick-add]').evaluate((element, row) => Boolean(element.compareDocumentPosition(row) & Node.DOCUMENT_POSITION_FOLLOWING), await windowRow.elementHandle())).toBe(true);
    expect(await windowRow.evaluate((element, timeline) => Boolean(element.compareDocumentPosition(timeline) & Node.DOCUMENT_POSITION_FOLLOWING), await viewport.elementHandle())).toBe(true);
    await expect(page.locator('.horizon-heading')).toHaveText('Horizon');
    await expect(page.locator('.horizon-calendar-symbol, .horizon-lane-symbol')).toHaveCount(0);

    await expect(page.locator('[data-calendar-id="CALBIRTH"] [data-lane-id]')).toHaveCount(3);
    await expect(page.locator('[data-calendar-id="CALHOLID"] [data-lane-id]')).toHaveCount(2);
    await expect(page.locator('[data-calendar-id="CALTRAVL"] [data-lane-id]')).toHaveCount(1);
    await expect(page.locator('[data-calendar-id="CALTRAVL"] [data-marker-id]')).toHaveCount(2);
    await expect(page.locator('[data-calendar-id="CALWORK0"] [data-lane-id]')).toHaveCount(3);
    await expect(page.locator('[data-calendar-id="CALWORK0"] [data-marker-id]')).toHaveCount(3);

    const scaleTrackBox = await page.locator('.horizon-scale-track').boundingBox();
    const todayLabelBox = await page.locator('.horizon-today-label').boundingBox();
    expect(scaleTrackBox).not.toBeNull();
    expect(todayLabelBox).not.toBeNull();
    expect(todayLabelBox.x).toBeGreaterThanOrEqual(scaleTrackBox.x - 1);
    expect(todayLabelBox.x + todayLabelBox.width).toBeLessThanOrEqual(scaleTrackBox.x + scaleTrackBox.width + 1);

    const openLane = page.locator('[data-lane-id="LANWAIT0"]');
    await expect(openLane.locator('[data-marker-id]')).toHaveCount(0);
    const openTrackBox = await openLane.locator('.horizon-lane-track').boundingBox();
    const openLineBox = await openLane.locator('.horizon-lane-line.is-open').boundingBox();
    expect(openTrackBox).not.toBeNull();
    expect(openLineBox).not.toBeNull();
    expect(Math.abs(openLineBox.x - openTrackBox.x)).toBeLessThanOrEqual(1);
    expect(Math.abs((openLineBox.x + openLineBox.width) - (openTrackBox.x + openTrackBox.width))).toBeLessThanOrEqual(1);
    expect(openLineBox.height).toBe(16);

    const openEndStyle = await openLane.locator('.horizon-lane-line').evaluate((line) => getComputedStyle(line, '::after').borderLeftStyle);
    expect(openEndStyle).toBe('solid');
    const continuingEndStyle = await page.locator('[data-lane-id="LANLONG0"] .horizon-lane-line.is-continuing').evaluate((line) => getComputedStyle(line, '::after').borderLeftStyle);
    expect(continuingEndStyle).toBe('solid');

    const markerTargetBox = await page.locator('[data-marker-id]').first().boundingBox();
    const markerDotBox = await page.locator('.horizon-marker-dot').first().boundingBox();
    expect(markerTargetBox).not.toBeNull();
    expect(markerDotBox).not.toBeNull();
    expect(markerTargetBox.width).toBeGreaterThanOrEqual(44);
    expect(markerTargetBox.height).toBeGreaterThanOrEqual(44);
    expect(markerDotBox.width).toBeGreaterThanOrEqual(16);
    expect(markerDotBox.width).toBeLessThanOrEqual(20);
    expect(markerDotBox.height).toBeGreaterThanOrEqual(16);
    expect(markerDotBox.height).toBeLessThanOrEqual(20);

    const boundaryTrackBox = await page.locator('[data-lane-id="LANBOUND"] .horizon-lane-track').boundingBox();
    const boundaryLine = page.locator('[data-lane-id="LANBOUND"] .horizon-lane-line.is-marker-terminated');
    const boundaryLineBox = await boundaryLine.boundingBox();
    const boundaryTarget = page.locator('[data-marker-id="EVTBOUND"]');
    const boundaryTargetBox = await boundaryTarget.boundingBox();
    const boundaryLabelBox = await page.locator('[data-lane-id="LANBOUND"] .horizon-lane-label').boundingBox();
    const boundaryTitleBox = await page.locator('[data-lane-id="LANBOUND"] .horizon-lane-title').boundingBox();
    const boundaryManageBox = await page.locator('[data-lane-id="LANBOUND"] .horizon-lane-controls > summary').boundingBox();
    expect(boundaryTrackBox).not.toBeNull();
    expect(boundaryLineBox).not.toBeNull();
    expect(boundaryTargetBox).not.toBeNull();
    expect(boundaryLabelBox).not.toBeNull();
    expect(boundaryTitleBox).not.toBeNull();
    expect(boundaryManageBox).not.toBeNull();
    expect(boundaryTargetBox.x).toBeGreaterThanOrEqual(boundaryTrackBox.x - 1);
    expect(boundaryTargetBox.x + boundaryTargetBox.width).toBeLessThanOrEqual(boundaryTrackBox.x + boundaryTrackBox.width + 1);
    expect(Math.abs((boundaryLineBox.x + boundaryLineBox.width) - (boundaryTargetBox.x + boundaryTargetBox.width / 2))).toBeLessThanOrEqual(1);
    await expect(boundaryTarget).toHaveClass(/is-lane-terminal/);
    await expect(boundaryTarget.locator('.horizon-marker-dot')).toHaveCount(0);
    const terminalGeometry = await boundaryLine.evaluate((line) => {
        const style = getComputedStyle(line, '::after');
        return {
            borderStyle: style.borderStyle,
            content: style.content,
            height: Number.parseFloat(style.height),
            width: Number.parseFloat(style.width),
        };
    });
    expect(terminalGeometry.borderStyle).toBe('solid');
    expect(terminalGeometry.content).toBe('""');
    expect(terminalGeometry.height).toBe(18);
    expect(terminalGeometry.width).toBe(18);
    expect(boundaryTitleBox.x).toBeGreaterThanOrEqual(boundaryLabelBox.x);
    expect(boundaryTitleBox.x + boundaryTitleBox.width).toBeLessThanOrEqual(boundaryLabelBox.x + boundaryLabelBox.width + 1);
    expect(boundaryManageBox.x + boundaryManageBox.width).toBeLessThanOrEqual(boundaryLabelBox.x + boundaryLabelBox.width + 1);
    const boundaryTitle = page.locator('[data-lane-id="LANBOUND"] .horizon-lane-title');
    await expect(boundaryTitle).toHaveText('Quarterly estimated tax payment deadline');
    const boundaryTitleOverflow = await boundaryTitle.evaluate((title) => ({
        horizontal: title.scrollWidth - title.clientWidth,
        vertical: title.scrollHeight - title.clientHeight,
    }));
    expect(boundaryTitleOverflow.horizontal).toBeLessThanOrEqual(1);
    expect(boundaryTitleOverflow.vertical).toBeLessThanOrEqual(1);

    await page.locator('[data-marker-id="EVTFLGHT"]').focus();
    await expect(page.locator('[data-marker-details="EVTFLGHT"]')).toBeVisible();
    await expect(page.locator('[data-marker-details="EVTFLGHT"] a', {hasText: 'Event controls'})).toHaveAttribute('href', '/events/?event_id=EVTFLGHT');
    await expect(page.locator('[data-marker-details="EVTFLGHT"] a', {hasText: 'RSVP controls'})).toHaveAttribute('href', '/rsvps/?event_id=EVTFLGHT');
});

test('persists complete calendar visibility for the organizer', async ({context, page}) => {
    const birthdaysToggle = page.locator('[data-calendar-toggle="CALBIRTH"]');
    await birthdaysToggle.uncheck();
    await expect(page.locator('[data-calendar-id="CALBIRTH"]')).toBeHidden();
    await expect(page.locator('[data-calendar-id="CALHOLID"]')).toBeVisible();

    await context.clearCookies();
    await page.goto('/browser-login/');
    await expect(page.locator('[data-calendar-toggle="CALBIRTH"]')).not.toBeChecked();
    await expect(page.locator('[data-calendar-id="CALBIRTH"]')).toBeHidden();
    await expect(page.locator('[data-calendar-control="CALBIRTH"]')).toBeVisible();

    await page.locator('[data-calendar-toggle="CALBIRTH"]').check();
    await expect(page.locator('[data-calendar-id="CALBIRTH"]')).toBeVisible();
});

test('keeps each calendar color stable when visibility and the calendar set change', async ({page}) => {
    /** @type {string[]} */
    const createdCalendarURLs = [];
    const createCalendar = async (index) => page.evaluate(async (calendarIndex) => {
        const response = await fetch('/calendars/', {
            method: 'POST',
            credentials: 'same-origin',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({name: `Color ${calendarIndex}`, color_token: `color-${calendarIndex}`, timezone: 'America/Los_Angeles'}),
        });
        return {status: response.status, location: response.headers.get('Location')};
    }, index);

    const visibleBefore = await page.locator('[data-calendar-toggle]:checked').count();
    for (let calendarIndex = visibleBefore; calendarIndex < 8; calendarIndex += 1) {
        const result = await createCalendar(calendarIndex);
        expect(result.status).toBe(201);
        expect(result.location).toBeTruthy();
        createdCalendarURLs.push(result.location);
    }
    const overflow = await createCalendar(8);
    expect(overflow.status).toBe(409);

    await page.reload();
    const readCalendarColors = () => page.locator('[data-calendar-control]').evaluateAll((controls) => Object.fromEntries(controls.map((control) => [
        control.getAttribute('data-calendar-control'),
        getComputedStyle(control).getPropertyValue('--calendar-color').trim(),
    ])));
    const colorsBeforeVisibilityChange = await readCalendarColors();
    const firstVisibleToggle = page.locator('[data-calendar-toggle]:checked').first();
    const hiddenCalendarID = await firstVisibleToggle.getAttribute('data-calendar-toggle');
    expect(hiddenCalendarID).toBeTruthy();
    const hiddenToggle = page.locator(`[data-calendar-toggle="${hiddenCalendarID}"]`);
    await hiddenToggle.uncheck();
    await expect(page.locator('[data-horizon-status]')).toHaveText(`Hid calendar ${hiddenCalendarID}.`);
    expect(await readCalendarColors()).toEqual(colorsBeforeVisibilityChange);
    await hiddenToggle.check();
    await expect(page.locator('[data-horizon-status]')).toHaveText(`Showed calendar ${hiddenCalendarID}.`);
    expect(await readCalendarColors()).toEqual(colorsBeforeVisibilityChange);
    await hiddenToggle.uncheck();
    await expect(page.locator('[data-horizon-status]')).toHaveText(`Hid calendar ${hiddenCalendarID}.`);
    expect(await readCalendarColors()).toEqual(colorsBeforeVisibilityChange);
    const replacement = await createCalendar(9);
    expect(replacement.status).toBe(201);
    expect(replacement.location).toBeTruthy();
    createdCalendarURLs.push(replacement.location);

    await page.reload();
    await expect(page.locator('[data-calendar-toggle]:checked')).toHaveCount(8);
    expect(await page.locator('[data-calendar-toggle]').count()).toBeGreaterThan(8);
    const colorsAfterCalendarAddition = await readCalendarColors();
    for (const [calendarID, color] of Object.entries(colorsBeforeVisibilityChange)) {
        expect(colorsAfterCalendarAddition[calendarID]).toBe(color);
    }
    expect(Object.values(colorsAfterCalendarAddition).every((color) => color !== '')).toBe(true);

    await page.evaluate(async ({calendarURLs, restoreCalendarID}) => {
        for (const calendarURL of calendarURLs) {
            const response = await fetch(calendarURL, {method: 'DELETE', credentials: 'same-origin'});
            if (!response.ok) {
                throw new Error(`Delete test calendar failed with ${response.status}.`);
            }
        }
        const response = await fetch(`/calendars/${restoreCalendarID}`, {
            method: 'PATCH',
            credentials: 'same-origin',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify({visible: true}),
        });
        if (!response.ok) {
            throw new Error(`Restore calendar visibility failed with ${response.status}.`);
        }
    }, {calendarURLs: createdCalendarURLs, restoreCalendarID: hiddenCalendarID});
});

test('opens global settings from a compact avatar menu', async ({page}) => {
    const userMenu = page.locator('[data-user-menu]');
    const dialog = page.locator('[data-settings-dialog]');

    await expect(page.locator('main [data-settings-dialog]')).toHaveCount(0);
    await expect(userMenu.getByRole('img', {name: 'User avatar'})).toBeVisible();
    await expect(dialog).toBeHidden();

    await userMenu.locator(':scope > summary').click();
    await expect(userMenu.getByRole('button', {name: 'Settings', exact: true})).toBeVisible();
    await expect(userMenu.getByRole('button', {name: 'Sign Out', exact: true})).toBeVisible();
    await expect(dialog).toBeHidden();

    await openSettings(page);
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('heading', {name: 'Create calendar'})).toBeVisible();
    await expect(dialog.getByRole('heading', {name: 'Create lane'})).toBeVisible();
    await expect(page).toHaveURL(/#settings\/calendars-lanes$/);

    await dialog.getByRole('tab', {name: 'Integrations'}).click();
    await expect(dialog.getByRole('heading', {name: 'Google Calendar'}).first()).toBeVisible();
    await expect(page).toHaveURL(/#settings\/integrations$/);
    await page.keyboard.press('ArrowRight');
    await expect(dialog.getByRole('tab', {name: 'Help'})).toHaveAttribute('aria-selected', 'true');
    await expect(page).toHaveURL(/#settings\/help$/);
    await expect(dialog.getByRole('heading', {name: 'Horizon keyboard controls'})).toBeVisible();
    await expect(dialog.locator('[data-settings-panel="help"]')).toContainText('Pan backward or forward.');
    await expect(dialog.locator('[data-settings-panel="help"]')).toContainText('Select the day, week, month, or year scale.');
    await expect(dialog.locator('[data-settings-panel="help"]')).toContainText('Show or hide a calendar.');
    await expect(dialog.locator('[data-settings-panel="help"]')).not.toContainText('Scale out or in.');
    await expect(page.getByText('Keyboard: H/L pan')).toHaveCount(0);
    await expect(dialog.getByRole('textbox', {name: 'Symbol'})).toHaveCount(0);
    await page.goto('/horizon/#settings/help');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('tab', {name: 'Help'})).toHaveAttribute('aria-selected', 'true');
    await expect(page).toHaveURL(/#settings\/help$/);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(userMenu.locator(':scope > summary')).toBeFocused();

    await page.goto('/events/');
    await openSettings(page);
    await expect(page.locator('[data-settings-dialog]')).toBeVisible();
    await closeSettings(page);

    await page.goto('/venues/');
    await openSettings(page);
    await expect(page.locator('[data-settings-dialog]')).toBeVisible();
});

test('changes and persists the organizer timezone in account settings', async ({page}) => {
    await openSettings(page, 'Account');
    const dialog = page.locator('[data-settings-dialog]');
    const timezoneInput = dialog.locator('[data-settings-organizer-timezone]');

    await expect(page).toHaveURL(/#settings\/account$/);
    await expect(timezoneInput).toHaveValue('America/Los_Angeles');

    await timezoneInput.fill('Local');
    await dialog.getByRole('button', {name: 'Save timezone'}).click();
    await expect(dialog.locator('[data-settings-status]')).toHaveText('Timezone is invalid.');
    await expect(timezoneInput).toHaveValue('Local');

    await timezoneInput.fill('America/New_York');
    await dialog.getByRole('button', {name: 'Save timezone'}).click();
    await expect(page).toHaveURL(/#settings\/account$/);
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('[data-settings-organizer-timezone]')).toHaveValue('America/New_York');
    await expect(page.locator('.horizon-window')).toContainText('America/New_York');

    await dialog.locator('[data-settings-organizer-timezone]').fill('America/Los_Angeles');
    await dialog.getByRole('button', {name: 'Save timezone'}).click();
    await expect(page).toHaveURL(/#settings\/account$/);
    await expect(dialog.locator('[data-settings-organizer-timezone]')).toHaveValue('America/Los_Angeles');
    await expect(page.locator('.horizon-window')).toContainText('America/Los_Angeles');
});

test('creates, reorders, resolves, and persists calendar lanes', async ({context, page}) => {
    await openSettings(page);
    const calendarForm = page.locator('form[data-settings-resource-form][data-resource-url="/calendars/"]');
    await calendarForm.locator('[name="name"]').fill('Browser Calendar');
    await calendarForm.locator('[name="color_token"]').fill('browser-calendar');
    await calendarForm.getByRole('button', {name: 'Create calendar'}).click();
    await closeSettings(page);

    await expect(page.locator('[data-calendar-toggle]').last()).toHaveAttribute('data-calendar-toggle', /.+/);
    await expect(page.locator('.horizon-calendar-toggle').last()).toContainText('Browser Calendar');
    const calendarID = await page.locator('[data-calendar-toggle]').last().getAttribute('data-calendar-toggle');
    expect(calendarID).toBeTruthy();

    await openSettings(page);
    const calendarManagement = page.locator(`[data-settings-calendar="${calendarID}"]`);
    await calendarManagement.getByRole('button', {name: 'Move up'}).click();
    await closeSettings(page);
    await expect(page.locator('[data-calendar-toggle]').nth(4)).toHaveAttribute('data-calendar-toggle', calendarID);

    await openSettings(page);
    const laneForm = page.locator('form[data-settings-resource-form][data-resource-url="/lanes/"]');
    await laneForm.locator('[name="calendar_id"]').selectOption(calendarID);
    await laneForm.locator('[name="title"]').fill('Browser open lane');
    await laneForm.getByRole('button', {name: 'Create lane'}).click();
    await closeSettings(page);
    const openLane = page.locator(`[data-calendar-id="${calendarID}"] [data-lane-id]`, {hasText: 'Browser open lane'});
    await expect(openLane).toBeVisible();
    await expect(openLane).toHaveAttribute('data-lane-open', 'true');

    await openSettings(page);
    await laneForm.locator('[name="calendar_id"]').selectOption(calendarID);
    await laneForm.locator('[name="title"]').fill('Browser finite lane');
    await laneForm.locator('[name="kind"]').selectOption('finite');
    const localEnd = await page.evaluate(() => {
        const value = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000);
        value.setMinutes(value.getMinutes() - value.getTimezoneOffset());
        return value.toISOString().slice(0, 16);
    });
    await laneForm.locator('[name="ends_at"]').fill(localEnd);
    await laneForm.getByRole('button', {name: 'Create lane'}).click();
    await closeSettings(page);
    const finiteLane = page.locator(`[data-calendar-id="${calendarID}"] [data-lane-id]`, {hasText: 'Browser finite lane'});
    await expect(finiteLane).toBeVisible();
    await expect(finiteLane).toHaveAttribute('data-lane-open', 'false');

    await finiteLane.locator('.horizon-lane-controls > summary').click();
    await finiteLane.getByRole('button', {name: 'Move lane up'}).click();
    await expect(page.locator(`[data-calendar-id="${calendarID}"] .horizon-lane-title`)).toHaveText(['Browser finite lane', 'Browser open lane']);

    await context.clearCookies();
    await page.goto('/browser-login/');
    await expect(page.locator('[data-calendar-toggle]').nth(4)).toHaveAttribute('data-calendar-toggle', calendarID);
    await expect(page.locator(`[data-calendar-id="${calendarID}"] .horizon-lane-title`).first()).toHaveText('Browser finite lane');

    const persistedOpenLane = page.locator(`[data-calendar-id="${calendarID}"] [data-lane-id]`, {hasText: 'Browser open lane'});
    await persistedOpenLane.locator('.horizon-lane-controls > summary').click();
    await persistedOpenLane.getByRole('button', {name: 'Resolve lane'}).click();
    await expect(page.locator(`[data-calendar-id="${calendarID}"] [data-lane-id]`, {hasText: 'Browser open lane'})).toHaveAttribute('data-lane-open', 'false');
});

test('shows and completes a durable attention probe', async ({page}) => {
    const waitingLane = page.locator('[data-lane-id="LANATTN0"]');
    await waitingLane.locator('.horizon-lane-controls > summary').click();
    await expect(waitingLane.getByText('Next attention:')).toBeVisible();
    const escalationInput = waitingLane.getByLabel('Escalation interval seconds');
    await expect(escalationInput).toHaveValue('86400');
    await escalationInput.fill('');
    await waitingLane.getByRole('button', {name: 'Save attention'}).click();
    await page.waitForLoadState('networkidle');
    await waitingLane.locator('.horizon-lane-controls > summary').click();
    await expect(waitingLane.getByLabel('Escalation interval seconds')).toHaveValue('');
    const pendingProbe = waitingLane.locator('[data-probe-state="pending"]');
    await expect(pendingProbe).toBeVisible();
    await pendingProbe.focus();
    await page.keyboard.press('Enter');
    await waitingLane.getByRole('button', {name: 'Complete probe'}).click();

    await expect(waitingLane.locator('[data-probe-state="completed"]')).toBeVisible();
    await expect(waitingLane.locator('[data-probe-state="pending"]')).toBeVisible();
});

test('connects Google Calendar and retains its calendar groups automatically', async ({context, page}) => {
	await context.clearCookies();
	await page.goto('/browser-new-login/');
	if (await page.locator('[data-horizon-setup]').count() !== 0) {
		await page.getByRole('button', {name: 'Start Horizon'}).click();
	}
	await expect(page.locator('.horizon-calendar-toggle', {hasText: 'Personal'})).toHaveCount(1);
    await openSettings(page, 'Integrations');
    await page.getByRole('button', {name: 'Connect Google Calendar'}).click();
    await expect(page.locator('[data-calendar-confirmation]')).toBeVisible();
    await expect(page.getByText('Consent verified')).toBeVisible();
	    await expect(page.getByRole('heading', {name: 'Read-only access'})).toBeVisible();
	    await expect(page.getByRole('heading', {name: 'Confirm Google Calendar'})).toBeVisible();
	    await page.getByRole('button', {name: 'Create connection'}).click();
	    await expect(page.locator('[data-calendar-task-status]')).toContainText('Calendar import task');
	    await expect(page).toHaveURL(/\/horizon\/#settings\/integrations$/);

		await expect(page.getByText('Connected', {exact: true})).toBeVisible();
		await expect(page.locator('[data-calendar-task-state]')).toHaveText('Running', {timeout: 4000});
		await expect(page.locator('[data-calendar-task-state]')).toHaveText('Complete', {timeout: 10000});
	await expect(page.getByText('RSVP imports Google calendars and birthdays automatically.')).toBeVisible();
	await expect(page.getByRole('button', {name: 'Select source calendars'})).toHaveCount(0);

	await closeSettings(page);

	const birthdayToggle = page.locator('.horizon-calendar-toggle', {hasText: 'Birthdays'}).locator('[data-calendar-toggle]');
	const holidayToggle = page.locator('.horizon-calendar-toggle', {hasText: 'Holidays'}).locator('[data-calendar-toggle]');
	const familyToggle = page.locator('.horizon-calendar-toggle', {hasText: 'Family'}).locator('[data-calendar-toggle]');
	const primaryToggle = page.locator('.horizon-calendar-toggle', {hasText: 'temirov@gmail.com'}).locator('[data-calendar-toggle]');
	await expect(birthdayToggle).toBeChecked();
	await expect(holidayToggle).toBeChecked();
	await expect(familyToggle).not.toBeChecked();
	const birthdayCalendarID = await birthdayToggle.getAttribute('data-calendar-toggle');
	const holidayCalendarID = await holidayToggle.getAttribute('data-calendar-toggle');
	const familyCalendarID = await familyToggle.getAttribute('data-calendar-toggle');
	const primaryCalendarID = await primaryToggle.getAttribute('data-calendar-toggle');
	await expect(page.locator(`[data-calendar-id="${holidayCalendarID}"] [data-lane-id]`)).toHaveCount(2);
	await expect(page.locator(`[data-calendar-id="${familyCalendarID}"] [data-lane-id]`)).toHaveCount(1);
	await expect(page.locator(`[data-calendar-id="${primaryCalendarID}"] [data-lane-id]`, {hasText: 'Provider future event'})).toHaveCount(1);
	await expect(page.locator(`[data-calendar-id="${birthdayCalendarID}"] [data-lane-id]`, {hasText: 'Happy birthday!'})).toHaveCount(1);
	await expect(page.locator(`[data-calendar-id="${primaryCalendarID}"] [data-lane-id]`, {hasText: 'Happy birthday!'})).toHaveCount(0);
	await expect(page.locator(`[data-calendar-id="${primaryCalendarID}"] [data-lane-id]`, {hasText: 'Contact anniversary'})).toHaveCount(1);
	await expect(page.locator('.horizon-calendar-toggle', {hasText: 'Contacts birthdays'})).toHaveCount(0);
	await expect(page.locator('.horizon-calendar-toggle', {hasText: 'providerFutureType'})).toHaveCount(0);
	await expect(page.locator(`[data-calendar-id="${familyCalendarID}"]`)).toBeHidden();
	await expect(page.locator('.horizon-calendar-toggle', {hasText: 'Personal'})).toHaveCount(0);
	const presentationColors = await page.locator('.horizon-calendar-toggle:has([data-calendar-toggle]:checked)').evaluateAll((controls) => controls.map((control) => getComputedStyle(control).getPropertyValue('--calendar-color').trim()));
	expect(new Set(presentationColors).size).toBe(presentationColors.length);
	await expect(page.getByRole('button', {name: /Synchronize/})).toHaveCount(0);
	await expect.poll(async () => {
		await page.waitForTimeout(500);
		await page.reload();
		return page.locator('[data-lane-id]', {hasText: 'Ada provider birthday updated'}).count();
	}, {timeout: 10000}).toBe(1);
	await expect(page.locator(`[data-calendar-id="${birthdayCalendarID}"] [data-lane-id]`)).toHaveCount(3);
	await expect(page.locator('[data-lane-id]', {hasText: 'Lin provider birthday'})).toHaveCount(0);
	await expect(page.locator('[data-lane-id]', {hasText: 'Maya provider birthday'})).toHaveCount(0);
	await expect(page.locator(`[data-calendar-id="${birthdayCalendarID}"] [data-lane-id]`, {hasText: 'Primary review birthday'})).toHaveCount(1);
	await expect(page.locator(`[data-calendar-id="${primaryCalendarID}"] [data-lane-id]`, {hasText: 'Primary review'})).toHaveCount(0);
	await expect(familyToggle).not.toBeChecked();

    await openSettings(page, 'Integrations');
    await expect(page.locator('[data-calendar-sync-state]')).not.toBeEmpty();
    await expect(page.locator('[data-calendar-last-success]')).toHaveAttribute('datetime', /^\d{4}-\d{2}-\d{2}T/);
    await page.getByRole('button', {name: 'Disconnect Google Calendar'}).click();
    await page.waitForLoadState('networkidle');
    await openSettings(page, 'Integrations');
    await expect(page.getByRole('button', {name: 'Connect Google Calendar'})).toBeVisible();
});

test('supports keyboard pan, scale, visibility, and marker selection', async ({page}) => {
    const viewport = page.locator('[data-horizon-viewport]');
    const horizonView = page.locator('[data-horizon-view]');
    for (const scaleName of ['Day', 'Week', 'Month', 'Year']) {
        const scale = scaleName.toLowerCase();
        await page.getByRole('button', {name: `${scaleName} scale`}).click();
        await expect(horizonView).toHaveAttribute('data-horizon-scale', scale);
        await expect(page.getByRole('button', {name: `${scaleName} scale`})).toHaveAttribute('aria-pressed', 'true');
        await expect(page.locator('[data-scale-preset][aria-pressed="true"]')).toHaveCount(1);
        const [windowStart, windowEnd] = await readHorizonLocalDates(page);
        expect(windowEnd).toBe(addHorizonScale(windowStart, scale));
    }

    await page.goto('/horizon/');
    await expect(horizonView).toHaveAttribute('data-horizon-scale', 'year');
    await expect(page.getByRole('button', {name: 'Year scale'})).toHaveAttribute('aria-pressed', 'true');
    const [storedStart, storedEnd] = await readHorizonLocalDates(page);
    expect(storedEnd).toBe(addHorizonScale(storedStart, 'year'));

    for (const [key, scaleName] of [['d', 'Day'], ['w', 'Week'], ['m', 'Month'], ['y', 'Year']]) {
        await viewport.focus();
        await page.keyboard.press(key);
        await expect(horizonView).toHaveAttribute('data-horizon-scale', scaleName.toLowerCase());
        await expect(page.getByRole('button', {name: `${scaleName} scale`})).toHaveAttribute('aria-pressed', 'true');
    }
    await viewport.focus();
    await page.keyboard.press('=');
    await page.keyboard.press('-');
    await expect(horizonView).toHaveAttribute('data-horizon-scale', 'year');

    await viewport.focus();
    await page.keyboard.press('m');
    await expect(horizonView).toHaveAttribute('data-horizon-scale', 'month');
    const [monthStart, monthEnd] = await readHorizonLocalDates(page);
    await viewport.focus();
    await page.keyboard.press('l');
    await expect(horizonView).toHaveAttribute('data-horizon-scale', 'month');
    const [nextMonthStart, nextMonthEnd] = await readHorizonLocalDates(page);
    expect(nextMonthStart).toBe(monthEnd);
    expect(nextMonthEnd).toBe(addHorizonScale(nextMonthStart, 'month'));
    await viewport.focus();
    await page.keyboard.press('h');
    await expect.poll(readHorizonLocalDates.bind(null, page)).toEqual([monthStart, monthEnd]);

    await viewport.focus();
    await page.keyboard.press('2');
    await expect(page.locator('[data-calendar-id="CALHOLID"]')).toBeHidden();
    await viewport.focus();
    await page.keyboard.press('j');
    await expect(page.locator('[data-marker-id].is-selected')).toHaveCount(1);
    await expect(page.locator('[data-marker-id].is-selected')).toBeFocused();
    await viewport.focus();
    await page.keyboard.press('2');
    await expect(page.locator('[data-calendar-id="CALHOLID"]')).toBeVisible();
});

test('provides labeled controls, focus order, and color-independent meaning', async ({page}) => {
    await page.locator('[data-quick-add] > summary').click();
    const visibleFields = page.locator('[data-quick-add] input:not([type="hidden"]):visible, [data-quick-add] select:visible, [data-quick-add] textarea:visible');
    const fieldCount = await visibleFields.count();
    for (let fieldIndex = 0; fieldIndex < fieldCount; fieldIndex += 1) {
        await expect(visibleFields.nth(fieldIndex).locator('xpath=ancestor::label[1]')).toHaveCount(1);
    }
    const parserInput = page.locator('form:has([name="source"][value="natural_language"]) [name="input_text"]');
    await parserInput.focus();
    await parserInput.pressSequentially('high-jolt link 1+1=2');
    await expect(parserInput).toHaveValue('high-jolt link 1+1=2');
    await parserInput.clear();
    await page.keyboard.press('Tab');
    await expect(page.locator('form:has([name="source"][value="natural_language"]) [name="calendar_id"]')).toBeFocused();
    await page.keyboard.press('Tab');
    await expect(page.getByRole('button', {name: 'Parse into draft'})).toBeFocused();

    await expect(page.locator('[data-calendar-control="CALBIRTH"]')).toContainText('Birthdays');
    await expect(page.locator('[data-lane-id="LANWAIT0"]')).toHaveAttribute('data-lane-open', 'true');
    await expect(page.locator('[data-marker-id="EVTFLGHT"]')).toHaveAttribute('aria-label', /event marker/);
});

test('creates and deletes a derived marker on its anchor lane', async ({page}) => {
    const anchor = page.getByRole('button', {name: 'Flight departs. event marker.'});
    await anchor.focus();
    await page.getByRole('button', {name: 'Add derived marker'}).click();

    const derived = page.getByRole('button', {name: 'Derived marker. derived marker.'});
    await expect(derived).toBeVisible();
    const anchorLaneID = await anchor.locator('xpath=ancestor::*[@data-lane-id]').getAttribute('data-lane-id');
    const derivedLaneID = await derived.locator('xpath=ancestor::*[@data-lane-id]').getAttribute('data-lane-id');
    expect(derivedLaneID).toBe(anchorLaneID);

    await derived.focus();
    await expect(page.getByText(/Anchor marker:/)).toBeVisible();
    await page.getByRole('button', {name: 'Delete derived marker'}).click();
    await expect(page.getByRole('button', {name: 'Derived marker. derived marker.'})).toHaveCount(0);
});

test('previews, corrects, confirms, and cancels quick ingestion drafts', async ({page}) => {
    await page.locator('[data-quick-add] > summary').click();
    const quickForm = page.locator('[data-quick-add-form]');
    await quickForm.locator('[name="mode"]').selectOption('open_lane');
    await quickForm.locator('[name="calendar_id"]').selectOption('CALWAIT0');
    await quickForm.locator('[name="title"]').fill('Browser draft waiting lane');
    await quickForm.locator('[name="review_interval_seconds"]').fill('604800');
    const nextProbe = await page.evaluate(() => {
        const value = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000);
        value.setMinutes(value.getMinutes() - value.getTimezoneOffset());
        return value.toISOString().slice(0, 16);
    });
    await quickForm.locator('[name="next_probe_at"]').fill(nextProbe);
    await quickForm.getByRole('button', {name: 'Create ingestion draft'}).click();

    await page.locator('[data-quick-add] > summary').click();
    const waitingDraft = page.locator('[data-ingestion-draft]', {hasText: 'Browser draft waiting lane'});
    await expect(waitingDraft).toBeVisible();
    await expect(waitingDraft.getByText(/Calendar: Waiting\. Lane: new lane/)).toBeVisible();
    await expect(waitingDraft.getByText(/Marker: none\. Attention: review every 604800 seconds/)).toBeVisible();
    await expect(page.locator('[data-lane-id]', {hasText: 'Browser draft waiting lane'})).toHaveCount(0);

    await waitingDraft.locator('[name="title"]').fill('Corrected browser waiting lane');
    await waitingDraft.locator('[name="timezone"]').fill('America/New_York');
    await waitingDraft.getByRole('button', {name: 'Save proposal'}).click();
    await page.locator('[data-quick-add] > summary').click();
    const correctedDraft = page.locator('[data-ingestion-draft]', {hasText: 'Corrected browser waiting lane'});
    await expect(correctedDraft.locator('[name="timezone"]')).toHaveValue('America/New_York');
    await expect(correctedDraft.locator('[name="next_probe_at"]')).toHaveValue(nextProbe);
    await correctedDraft.getByRole('button', {name: 'Confirm draft'}).click();
    await expect(page.locator('[data-calendar-id="CALWAIT0"] [data-lane-id]', {hasText: 'Corrected browser waiting lane'})).toBeVisible();

    await page.locator('[data-quick-add] > summary').click();
    await quickForm.locator('[name="mode"]').selectOption('dated_event');
    await quickForm.locator('[name="calendar_id"]').selectOption('CALBIRTH');
    await quickForm.locator('[name="title"]').fill('Canceled browser birthday');
    const eventStart = await page.evaluate(() => {
        const value = new Date(Date.now() + 14 * 24 * 60 * 60 * 1000);
        value.setMinutes(value.getMinutes() - value.getTimezoneOffset());
        return value.toISOString().slice(0, 16);
    });
    const eventEnd = await page.evaluate(() => {
        const value = new Date(Date.now() + 14 * 24 * 60 * 60 * 1000 + 60 * 60 * 1000);
        value.setMinutes(value.getMinutes() - value.getTimezoneOffset());
        return value.toISOString().slice(0, 16);
    });
    await quickForm.locator('[name="starts_at"]').fill(eventStart);
    await quickForm.locator('[name="ends_at"]').fill(eventEnd);
    await quickForm.getByRole('button', {name: 'Create ingestion draft'}).click();
    await page.locator('[data-quick-add] > summary').click();
    const canceledDraft = page.locator('[data-ingestion-draft]', {hasText: 'Canceled browser birthday'});
    await expect(canceledDraft.getByText(/Lane: new lane/)).toBeVisible();
    await expect(canceledDraft.getByText(/Marker: .* to .*\. Attention: none/)).toBeVisible();
    await canceledDraft.getByRole('button', {name: 'Cancel draft'}).click();
    await expect(page.locator('[data-lane-id]', {hasText: 'Canceled browser birthday'})).toHaveCount(0);
});

test('parses natural-language waiting, flight, and incomplete drafts', async ({page}) => {
    await page.locator('[data-quick-add] > summary').click();
    const parserForm = page.locator('form:has([name="source"][value="natural_language"])');
    await parserForm.locator('[name="calendar_id"]').selectOption('CALWAIT0');
    await parserForm.locator('[name="input_text"]').fill('unresolved appeal with weekly checks');
    await parserForm.getByRole('button', {name: 'Parse into draft'}).click();
    await page.locator('[data-quick-add] > summary').click();
    const waitingDraft = page.locator('[data-ingestion-draft]', {hasText: 'Unresolved appeal'});
    await expect(waitingDraft.getByText('Parser-inferred proposal.')).toBeVisible();
    await expect(waitingDraft.getByText(/Attention: review every 604800 seconds/)).toBeVisible();
    await expect(page.locator('[data-lane-id]', {hasText: 'Unresolved appeal'})).toHaveCount(0);
    await waitingDraft.getByRole('button', {name: 'Confirm draft'}).click();
    await expect(page.locator('[data-calendar-id="CALWAIT0"] [data-lane-id]', {hasText: 'Unresolved appeal'})).toBeVisible();

    await page.locator('[data-quick-add] > summary').click();
    await parserForm.locator('[name="calendar_id"]').selectOption('CALTRAVL');
    await parserForm.locator('[name="input_text"]').fill('flight with relative departure and arrival markers');
    await parserForm.getByRole('button', {name: 'Parse into draft'}).click();
    await page.locator('[data-quick-add] > summary').click();
    const flightDraft = page.locator('[data-ingestion-draft]', {hasText: 'Parsed flight'});
    await expect(flightDraft.getByText(/Relative marker rules: -7200 seconds from start; 3600 seconds from end/)).toBeVisible();
    await flightDraft.getByRole('button', {name: 'Confirm draft'}).click();
    const flightLane = page.locator('[data-calendar-id="CALTRAVL"] [data-lane-id]', {hasText: 'Parsed flight'});
    await expect(flightLane).toBeVisible();
    await expect(flightLane.locator('[data-marker-id]')).toHaveCount(3);
    await expect(flightLane.locator('[data-marker-type="derived"]')).toHaveCount(2);

    await page.locator('[data-quick-add] > summary').click();
    await parserForm.locator('[name="calendar_id"]').selectOption('CALTRAVL');
    await parserForm.locator('[name="input_text"]').fill('flight missing an end');
    await parserForm.getByRole('button', {name: 'Parse into draft'}).click();
    await page.locator('[data-quick-add] > summary').click();
    const incompleteDraft = page.locator('[data-ingestion-draft]', {hasText: 'Incomplete flight'});
    await expect(incompleteDraft.getByText('Missing required values: ends_at.')).toBeVisible();
    await expect(incompleteDraft.getByRole('button', {name: 'Confirm draft'})).toHaveCount(0);
    const completedEnd = await page.evaluate(() => {
        const value = new Date(Date.now() + 24 * 24 * 60 * 60 * 1000 + 2 * 60 * 60 * 1000);
        value.setMinutes(value.getMinutes() - value.getTimezoneOffset());
        return value.toISOString().slice(0, 16);
    });
    await incompleteDraft.locator('[name="ends_at"]').fill(completedEnd);
    await incompleteDraft.getByRole('button', {name: 'Save proposal'}).click();
    await page.locator('[data-quick-add] > summary').click();
    await expect(page.locator('[data-ingestion-draft]', {hasText: 'Incomplete flight'}).getByRole('button', {name: 'Confirm draft'})).toBeVisible();
});

test('restores independent-event calendar selection after cancel', async ({page}) => {
    await page.goto('/events/');
    await page.locator('#globalNewEventButton').click();
    const anchorSelect = page.locator('#anchorEventSelect');
    const calendarSelect = page.locator('#calendarSelect');
    await anchorSelect.selectOption({index: 1});
    await expect(calendarSelect).toBeDisabled();

    await page.locator('#cancelNewEventButton').click();
    await page.locator('#globalNewEventButton').click();
    await expect(anchorSelect).toHaveValue('');
    await expect(calendarSelect).toBeEnabled();
});

test('keeps the QR and public RSVP response flow available', async ({context, page}) => {
    await page.goto('/rsvps/qr/?rsvp_id=RSVPBR01');
    const qrImage = page.getByRole('img', {name: 'RSVP QR Code'});
    await expect(qrImage).toHaveAttribute('src', /^data:image\/png;base64,/);
    const publicLink = page.getByRole('link', {name: /\/response\/\?rsvp_id=RSVPBR01/});
    await expect(publicLink).toHaveAttribute('href', 'http://127.0.0.1:18080/response/?rsvp_id=RSVPBR01');

    await context.clearCookies();
    await page.goto('/response/?rsvp_id=RSVPBR01');
    await expect(page.getByRole('heading', {name: "You're Invited!"})).toBeVisible();
    await expect(page.getByText('Browser Terminal')).toBeVisible();
    await page.getByRole('button', {name: '+2 Guests'}).click();
    await expect(page).toHaveURL(/\/response\/thankyou\?rsvp_id=RSVPBR01$/);
    await expect(page.getByText(/Thank/)).toBeVisible();
});

test('renders the interactive view at the supported mobile width', async ({page}) => {
    await page.setViewportSize({width: 390, height: 844});
    await expect(page.getByRole('heading', {name: 'Horizon'})).toBeVisible();
    await expect(page.locator('[data-horizon-viewport]')).toBeVisible();
    const mobileWindowRow = page.locator('[data-horizon-window-row]');
    await expect(mobileWindowRow.locator('.horizon-window time')).toHaveCount(2);
    await expect(mobileWindowRow.locator('.horizon-view-controls button')).toHaveCount(6);
    for (const control of await mobileWindowRow.locator('.horizon-view-controls button').all()) {
        await expect(control).toBeVisible();
    }
    await expect(page.locator('[data-lane-id="LANWAIT0"] .horizon-lane-line.is-open')).toBeVisible();
    const markerTargetBox = await page.locator('[data-marker-id]').first().boundingBox();
    expect(markerTargetBox).not.toBeNull();
    expect(markerTargetBox.width).toBeGreaterThanOrEqual(44);
    expect(markerTargetBox.height).toBeGreaterThanOrEqual(44);
    const longLaneTitle = page.locator('[data-lane-id="LANBOUND"] .horizon-lane-title');
    await expect(longLaneTitle).toHaveText('Quarterly estimated tax payment deadline');
    const longLaneTitleOverflow = await longLaneTitle.evaluate((title) => ({
        horizontal: title.scrollWidth - title.clientWidth,
        vertical: title.scrollHeight - title.clientHeight,
    }));
    expect(longLaneTitleOverflow.horizontal).toBeLessThanOrEqual(1);
    expect(longLaneTitleOverflow.vertical).toBeLessThanOrEqual(1);
    const documentWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    expect(documentWidth).toBeLessThanOrEqual(390);
    const navigationBox = await page.locator('.fixed-navbar').boundingBox();
    const headingBox = await page.getByRole('heading', {name: 'Horizon'}).boundingBox();
    expect(navigationBox).not.toBeNull();
    expect(headingBox).not.toBeNull();
    expect(headingBox.y).toBeGreaterThanOrEqual(navigationBox.y + navigationBox.height);

    await page.locator('[data-user-menu] > summary').click();
    const menuBox = await page.locator('.user-menu-dropdown').boundingBox();
    expect(menuBox).not.toBeNull();
    expect(menuBox.x).toBeGreaterThanOrEqual(0);
    expect(menuBox.x + menuBox.width).toBeLessThanOrEqual(390);

    await page.getByRole('button', {name: 'Settings', exact: true}).click();
    const settingsBox = await page.locator('.settings-shell').boundingBox();
    expect(settingsBox).not.toBeNull();
    expect(settingsBox.x).toBeGreaterThanOrEqual(0);
    expect(settingsBox.x + settingsBox.width).toBeLessThanOrEqual(390);
    await expect(page.getByRole('tab', {name: 'Calendars & lanes'})).toBeVisible();
    await expect(page.getByRole('tab', {name: 'Integrations'})).toBeVisible();
});
