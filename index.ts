import { parse } from "csv-parse/sync";
import fs from "fs/promises";
import pLimit from "p-limit";
import * as path from "path";
import * as https from "https";
import { createWriteStream } from "fs";
import { unlink } from "fs/promises";
import { deprecate } from "util";

const outputPath = "/Users/antoni-ostrowski/Desktop/new-music";
const csvFilePath = "/Users/antoni-ostrowski/Desktop/yeat-tracker.csv";
const limit = pLimit(4);
const baseApiUrl = "https://api.pillows.su";

interface Track {
  era: string;
  title: string;
  notes: string;
  trackLength: string;
  fileDate: string;
  firstPreview: string;
  leakDate: string;
  ogFileLeakDate: string;
  type: string;
  availableLength: string;
  quality: string;
  links: string;
}

type ParsedCsv = string[][];

function convertArrayToObject(data: ParsedCsv): Track[] {
  return data.map((itemArray) => {
    const [
      era,
      title,
      notes,
      trackLength,
      fileDate,
      firstPreview,
      leakDate,
      ogFileLeakDate,
      type,
      availableLength,
      quality,
      links,
    ] = itemArray;

    return {
      era,
      title,
      notes,
      trackLength,
      fileDate,
      firstPreview,
      leakDate,
      ogFileLeakDate,
      type,
      availableLength,
      quality,
      links,
    } as Track;
  });
}

function trackIntoPromises(tracks: Track[]) {
  return tracks.flatMap((track) => {
    return limit(() => {
      const links = track.links.split("\n");

      const validDownloadLinks: string[] = [];
      links.forEach((link) => {
        if (isValidUrl(link)) {
          const id = getTrackIdFromUrl(link);
          const downloadLink = `${baseApiUrl}/api/download/${id}`;
          validDownloadLinks.push(downloadLink);
          return;
        }
      });
      // console.log("links for - ", track.title);
      // console.dir(validDownloadLinks, { depth: null });
      // Use flatMap to produce a single, flat array of ALL download promises.
      return validDownloadLinks.map((link) => {
        // The limiter controls the execution of each individual download
        return limit(() => downloadFileRecursive(link, track.title));
      });
      // console.log("links", links);
      // return downloadFileRecursive(track.links, track.title);
    });
  });
}

function downloadFileRecursive(
  url: string,
  fileName: string,
  finalFilePath?: string,
): Promise<string> {
  return new Promise(async (resolve, reject) => {
    const TIMEOUT_MS = 5000;
    //delay
    await new Promise((resolve) => setTimeout(resolve, 1000)); // 2000ms = 2 seconds

    console.log("starting for - ", url, fileName, finalFilePath);
    const req = https.get(url, (response) => {
      const redirectUrl = response.headers.location;
      const statusCode = response.statusCode;

      console.log({ statusCode, redirectUrl });
      if (statusCode && statusCode === 503) {
        console.log(`Received 503 for ${url}. Skipping file.`);
        response.resume(); // Must consume stream to clear the connection
        // Resolve with an empty string or a special code to signify skip
        return resolve("");
      }
      // --- 1. HANDLE REDIRECTS (3xx) ---
      if (statusCode && statusCode >= 300 && statusCode < 400 && redirectUrl) {
        console.log(`Received ${statusCode}. Redirecting to: ${redirectUrl}`);
        response.resume(); // Consume the stream data to free resources

        let fileType = getFileTypeFromUrl(redirectUrl);
        if (!fileType) {
          fileType = "mp3";
        }
        const nextUrl = baseApiUrl + redirectUrl;
        console.log("next url - ", nextUrl);
        downloadFileRecursive(
          nextUrl,
          fileName,
          `${outputPath}/${fileName}.${fileType}`,
        )
          .then(resolve)
          .catch(reject);
        return;
      }

      // --- 2. HANDLE NON-200 ERRORS (4xx, 5xx, etc.) ---
      if (statusCode !== 200 || !finalFilePath) {
        const errorMsg = finalFilePath
          ? `Download failed: Status Code ${statusCode}`
          : "No filename known after non-redirect status.";
        console.error(errorMsg);
        response.resume();
        return reject(new Error(errorMsg));
      }

      // --- 3. HANDLE SUCCESS (200 OK) ---

      // **CRITICAL FIX:** Create the stream NOW with the correct, known filename
      const fileStream = createWriteStream(finalFilePath);

      response.pipe(fileStream);

      fileStream.on("error", (err) => {
        console.error("File stream error:", err);
        fileStream.close();
        unlink(finalFilePath).catch(() => {});
        return reject(err);
      });

      fileStream.on("finish", () => {
        fileStream.close();
        console.log(`Download complete: ${finalFilePath}`);
        resolve(finalFilePath); // Resolve the promise with the final path
      });
    });

    req.on("error", (err) => {
      console.error("HTTPS request error:", err);
      return reject(err);
    });
    req.on("timeout", () => {
      console.error(
        `Request timeout after ${TIMEOUT_MS}ms for ${url}. Skipping file.`,
      );
      req.destroy(); // Abort the request
      resolve(""); // Resolve silently to skip the error/file
    });
    req.setTimeout(TIMEOUT_MS);
  });
}

function getTrackIdFromUrl(url: string) {
  const id = url.slice(-32);
  return id;
}
function getFileTypeFromUrl(url: string) {
  const fileTypeRegex: RegExp = /\.([^.]+)$/i;

  const matchResult = url.match(fileTypeRegex);

  if (matchResult && matchResult.length > 1) {
    // matchResult[1] holds the content of the first (and only) capture group,
    // which is the file extension (e.g., "mp3")
    return matchResult[1];
  }

  return null;
}

function isValidUrl(urlString: string) {
  try {
    return Boolean(new URL(urlString) && urlString.includes("pillows.su"));
  } catch (e) {
    return false;
  }
}

(async () => {
  const content = await fs.readFile(csvFilePath);
  const records = parse(content);

  // console.log({ records });
  // console.dir(records[6][1], { depth: null });
  const tracks = convertArrayToObject(records);
  // // records.forEach((record) => {
  // //   console.log(convertArrayToObject(record));
  // // });
  const itemsToRemove = 19;
  tracks.splice(tracks.length - itemsToRemove, itemsToRemove);
  const trackPromises = trackIntoPromises(tracks);
  console.log({ trackPromises });
  const r = await Promise.all(trackPromises);
  console.log({ r });
  // await downloadFileRecursive(
  //   `${baseApiUrl}/api/download/f59bc359e49bfb1b52ba32b8b2327719`,
  //   "example",
  // );
})();
