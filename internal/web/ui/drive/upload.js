function driveFileRel(file) {
  return String(file?.webkitRelativePath || file?.name || "").replaceAll("\\", "/").replace(/^\/+/, "");
}

function blockedDriveRel(rel) {
  const parts = String(rel || "").replaceAll("\\", "/").split("/").filter(Boolean);
  if (!parts.length) return true;
  for (const p of parts) {
    if (p === "." || p === ".." || p.startsWith(".") || /\.part$/i.test(p)) return true;
  }
  return false;
}

function readDirEntries(reader) {
  return new Promise((resolve, reject) => {
    const all = [];
    const next = () => {
      reader.readEntries((batch) => {
        if (!batch.length) {
          resolve(all);
          return;
        }
        all.push(...batch);
        next();
      }, reject);
    };
    next();
  });
}

async function walkDriveEntry(entry, prefix, files, dirs) {
  const rel = prefix ? `${prefix}/${entry.name}` : entry.name;
  if (blockedDriveRel(rel)) return;
  if (entry.isFile) {
    const file = await new Promise((resolve, reject) => entry.file(resolve, reject));
    files.push({ file, rel });
    return;
  }
  if (!entry.isDirectory) return;
  const children = await readDirEntries(entry.createReader());
  if (!children.length) {
    dirs.push(rel);
    return;
  }
  for (const child of children) {
    await walkDriveEntry(child, rel, files, dirs);
  }
}

function fromFileList(list) {
  return [...(list || [])]
    .map((file) => ({ file, rel: driveFileRel(file) }))
    .filter((item) => !blockedDriveRel(item.rel));
}

async function collectDriveDrop(dt) {
  try {
    const entries = [...(dt?.items || [])].map((item) => item.webkitGetAsEntry?.()).filter(Boolean);
    if (entries.length) {
      const files = [];
      const dirs = [];
      for (const entry of entries) {
        await walkDriveEntry(entry, "", files, dirs);
      }
      if (files.length || dirs.length) return { files, dirs };
    }
  } catch {
    // fall back to FileList
  }
  return { files: fromFileList(dt?.files), dirs: [] };
}

export { fromFileList, collectDriveDrop };
