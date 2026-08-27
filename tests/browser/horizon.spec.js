// @ts-check

const {test, expect} = require('@playwright/test');

test.beforeEach(async ({page}) => {
    await page.goto('/browser-login/');
    await expect(page).toHaveURL(/\/horizon\/$/);
    await expect(page.locator('[data-horizon-view]')).toBeVisible();
    await page.goto('/');
    await expect(page).toHaveURL(/\/horizon\/$/);
});

test('renders the lane and marker contract', async ({page}) => {
    await expect(page.locator('[data-calendar-id="CALBIRTH"] [data-lane-id]')).toHaveCount(3);
    await expect(page.locator('[data-calendar-id="CALHOLID"] [data-lane-id]')).toHaveCount(2);
    await expect(page.locator('[data-calendar-id="CALTRAVL"] [data-lane-id]')).toHaveCount(1);
    await expect(page.locator('[data-calendar-id="CALTRAVL"] [data-marker-id]')).toHaveCount(2);
    await expect(page.locator('[data-calendar-id="CALWORK0"] [data-lane-id]')).toHaveCount(3);
    await expect(page.locator('[data-calendar-id="CALWORK0"] [data-marker-id]')).toHaveCount(3);

    const openLane = page.locator('[data-lane-id="LANWAIT0"]');
    await expect(openLane.locator('[data-marker-id]')).toHaveCount(0);
    const openTrackBox = await openLane.locator('.horizon-lane-track').boundingBox();
    const openLineBox = await openLane.locator('.horizon-lane-line.is-open').boundingBox();
    expect(openTrackBox).not.toBeNull();
    expect(openLineBox).not.toBeNull();
    expect(Math.abs(openLineBox.x - openTrackBox.x)).toBeLessThanOrEqual(1);
    expect(Math.abs((openLineBox.x + openLineBox.width) - (openTrackBox.x + openTrackBox.width))).toBeLessThanOrEqual(1);
    expect(openLineBox.height).toBeGreaterThanOrEqual(12);
    expect(openLineBox.height).toBeLessThanOrEqual(16);

    const openEndStyle = await openLane.locator('.horizon-lane-line').evaluate((line) => getComputedStyle(line, '::after').borderLeftStyle);
    const finiteEndStyle = await page.locator('.horizon-lane-line.is-finite').first().evaluate((line) => getComputedStyle(line, '::after').borderStyle);
    expect(openEndStyle).toBe('solid');
    expect(finiteEndStyle).toContain('solid');
    const continuingEndStyle = await page.locator('[data-lane-id="LANLONG0"] .horizon-lane-line.is-continuing').evaluate((line) => getComputedStyle(line, '::after').borderLeftStyle);
    expect(continuingEndStyle).toBe('solid');

    const markerTargetBox = await page.locator('[data-marker-id]').first().boundingBox();
    const markerDotBox = await page.locator('.horizon-marker-dot').first().boundingBox();
    expect(markerTargetBox).not.toBeNull();
    expect(markerDotBox).not.toBeNull();
    expect(markerTargetBox.width).toBeGreaterThanOrEqual(44);
    expect(markerTargetBox.height).toBeGreaterThanOrEqual(44);
    expect(markerDotBox.width).toBeGreaterThanOrEqual(20);
    expect(markerDotBox.width).toBeLessThanOrEqual(24);
    expect(markerDotBox.height).toBeGreaterThanOrEqual(20);
    expect(markerDotBox.height).toBeLessThanOrEqual(24);

    const boundaryTrackBox = await page.locator('[data-lane-id="LANBOUND"] .horizon-lane-track').boundingBox();
    const boundaryTargetBox = await page.locator('[data-marker-id="EVTBOUND"]').boundingBox();
    expect(boundaryTrackBox).not.toBeNull();
    expect(boundaryTargetBox).not.toBeNull();
    expect(boundaryTargetBox.x).toBeGreaterThanOrEqual(boundaryTrackBox.x - 1);
    expect(boundaryTargetBox.x + boundaryTargetBox.width).toBeLessThanOrEqual(boundaryTrackBox.x + boundaryTrackBox.width + 1);

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

test('supports keyboard pan, scale, visibility, and marker selection', async ({page}) => {
    const viewport = page.locator('[data-horizon-viewport]');
    const board = page.locator('[data-horizon-board]');
    await viewport.focus();
    const initialBoardWidth = await board.evaluate((element) => element.scrollWidth);
    await page.keyboard.press('=');
    const scaledBoardWidth = await board.evaluate((element) => element.scrollWidth);
    expect(scaledBoardWidth).toBeGreaterThan(initialBoardWidth);

    await page.keyboard.press('l');
    await expect.poll(() => viewport.evaluate((element) => element.scrollLeft)).toBeGreaterThan(0);
    await page.keyboard.press('2');
    await expect(page.locator('[data-calendar-id="CALHOLID"]')).toBeHidden();
    await page.keyboard.press('j');
    await expect(page.locator('[data-marker-id].is-selected')).toHaveCount(1);
    await expect(page.locator('[data-marker-id].is-selected')).toBeFocused();
    await page.keyboard.press('2');
    await expect(page.locator('[data-calendar-id="CALHOLID"]')).toBeVisible();
});

test('renders the interactive view at the supported mobile width', async ({page}) => {
    await page.setViewportSize({width: 390, height: 844});
    await expect(page.getByRole('heading', {name: 'Horizon'})).toBeVisible();
    await expect(page.locator('[data-horizon-viewport]')).toBeVisible();
    await expect(page.locator('[data-lane-id="LANWAIT0"] .horizon-lane-line.is-open')).toBeVisible();
    const markerTargetBox = await page.locator('[data-marker-id]').first().boundingBox();
    expect(markerTargetBox).not.toBeNull();
    expect(markerTargetBox.width).toBeGreaterThanOrEqual(44);
    expect(markerTargetBox.height).toBeGreaterThanOrEqual(44);
    const documentWidth = await page.evaluate(() => document.documentElement.scrollWidth);
    expect(documentWidth).toBeLessThanOrEqual(390);
    const navigationBox = await page.locator('.fixed-navbar').boundingBox();
    const headingBox = await page.getByRole('heading', {name: 'Horizon'}).boundingBox();
    expect(navigationBox).not.toBeNull();
    expect(headingBox).not.toBeNull();
    expect(headingBox.y).toBeGreaterThanOrEqual(navigationBox.y + navigationBox.height);
});
