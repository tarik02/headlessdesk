import { createHash } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';

const changesetPackage = 'headlessdesk';
const changesetDir = '.changeset';

const readUpgradeData = async () => {
    const dataFile = process.env.RENOVATE_POST_UPGRADE_COMMAND_DATA_FILE;

    if (!dataFile) {
        return [];
    }

    try {
        const raw = await readFile(dataFile, 'utf8');
        const parsed = JSON.parse(raw);

        return Array.isArray(parsed) ? parsed : [];
    } catch {
        return [];
    }
};

const getVersion = (upgrade, key) => {
    const version = upgrade[key];

    return typeof version === 'string' && version.length > 0 ? version : null;
};

const getUpgradeLine = (upgrade) => {
    const depName = getVersion(upgrade, 'depName');

    if (!depName) {
        return null;
    }

    const current = getVersion(upgrade, 'currentValue') ?? getVersion(upgrade, 'currentVersion');
    const next = getVersion(upgrade, 'newValue') ?? getVersion(upgrade, 'newVersion');

    if (current && next) {
        return `- ${depName}: ${current} -> ${next}`;
    }

    if (next) {
        return `- ${depName}: ${next}`;
    }

    return `- ${depName}`;
};

const getChangesetName = (upgrades) => {
    const branchName = process.env.RENOVATE_BRANCH ?? process.env.GITHUB_HEAD_REF;
    const fallbackName = createHash('sha256').update(JSON.stringify(upgrades)).digest('hex').slice(0, 16);

    if (branchName) {
        const branchSlug = branchName
            .toLowerCase()
            .replaceAll(/[^a-z0-9]+/g, '-')
            .replaceAll(/^-|-$/g, '')
            .slice(0, 80);

        return branchSlug.length > 0 ? branchSlug : fallbackName;
    }

    return fallbackName;
};

const upgrades = await readUpgradeData();
const updateLines = upgrades.map((upgrade) => getUpgradeLine(upgrade)).filter((line) => line !== null);
const body = updateLines.length > 0 ? ['update dependencies', '', ...updateLines].join('\n') : 'update dependencies';
const changeset = `---\n"${changesetPackage}": patch\n---\n\n${body}\n`;
const changesetName = getChangesetName(upgrades);

await mkdir(changesetDir, { recursive: true });
await writeFile(path.join(changesetDir, `${changesetName}.md`), changeset);
