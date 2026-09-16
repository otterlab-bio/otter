#!/usr/bin/env Rscript

arguments <- commandArgs(trailingOnly = TRUE)
if (length(arguments) != 8) {
  stop("usage: gate6-xenofilter-legacy.R GRAFT_BAM HOST_BAM DESTINATION THREADS MM_THRESHOLD UNMAPPED_PENALTY NM_TAG REPORT_JSON")
}

graft_bam <- normalizePath(arguments[[1]], mustWork = TRUE)
host_bam <- normalizePath(arguments[[2]], mustWork = TRUE)
destination_folder <- normalizePath(arguments[[3]], mustWork = TRUE)
thread_count <- as.integer(arguments[[4]])
mm_threshold <- as.integer(arguments[[5]])
unmapped_penalty <- as.integer(arguments[[6]])
nm_tag <- arguments[[7]]
report_path <- arguments[[8]]

if (is.na(thread_count) || thread_count < 1) {
  stop("THREADS must be a positive integer")
}
if (is.na(mm_threshold) || mm_threshold < 0) {
  stop("MM_THRESHOLD must be a non-negative integer")
}
if (is.na(unmapped_penalty) || unmapped_penalty < 0) {
  stop("UNMAPPED_PENALTY must be a non-negative integer")
}

suppressPackageStartupMessages(library(BiocParallel))
suppressPackageStartupMessages(library(XenofilteR))

sample_list <- data.frame(Graft = graft_bam, Host = host_bam, stringsAsFactors = FALSE)
bp_param <- BiocParallel::MulticoreParam(workers = thread_count, stop.on.error = TRUE)

XenofilteR::XenofilteR(
  sample.list = sample_list,
  destination.folder = destination_folder,
  bp.param = bp_param,
  output.names = basename(graft_bam),
  MM_threshold = mm_threshold,
  Unmapped_penalty = unmapped_penalty,
  NM_id = nm_tag
)

output_directory <- file.path(destination_folder, "Filtered_bams")
output_files <- list.files(output_directory, recursive = TRUE, full.names = TRUE)
report <- list(
  schema_version = "gate6.xenofilter-legacy-run/v1",
  graft_bam = graft_bam,
  host_bam = host_bam,
  destination_folder = destination_folder,
  output_directory = output_directory,
  thread_count = thread_count,
  mm_threshold = mm_threshold,
  unmapped_penalty = unmapped_penalty,
  nm_tag = nm_tag,
  xenofilter_version = as.character(utils::packageVersion("XenofilteR")),
  output_files = output_files
)
jsonlite::write_json(report, report_path, pretty = TRUE, auto_unbox = TRUE)
