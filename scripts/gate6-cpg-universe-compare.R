#!/usr/bin/env Rscript

arguments <- commandArgs(trailingOnly = TRUE)
if (length(arguments) != 3L) {
  stop("Usage: gate6-cpg-universe-compare.R <fasta> <cpg_ron> <output_dir>", call. = FALSE)
}

fasta_path <- normalizePath(arguments[[1L]], mustWork = TRUE)
ron_path <- normalizePath(arguments[[2L]], mustWork = TRUE)
output_directory <- normalizePath(arguments[[3L]], mustWork = FALSE)
dir.create(output_directory, recursive = TRUE, showWarnings = FALSE)

if (!requireNamespace("Biostrings", quietly = TRUE)) {
  stop("The Bioconductor package 'Biostrings' is required", call. = FALSE)
}

is_standard_chromosome <- function(contig_name) {
  cleaned_name <- sub("^chr", "", contig_name, ignore.case = TRUE)
  toupper(cleaned_name) %in% c(as.character(1:22), "X", "Y", "M", "MT")
}

read_fasta_cpgs <- function(path, output_path) {
  sequences <- Biostrings::readDNAStringSet(path, format = "fasta")
  sequence_names <- sub("\\s.*$", "", names(sequences))
  selected_indices <- which(vapply(sequence_names, is_standard_chromosome, logical(1)))
  selected_names <- sequence_names[selected_indices]
  output_connection <- file(output_path, open = "wt")
  on.exit(close(output_connection), add = TRUE)

  total_cpg_count <- 0L
  for (sequence_index in selected_indices) {
    sequence_name <- sequence_names[[sequence_index]]
    cpg_matches <- Biostrings::matchPattern("CG", sequences[[sequence_index]])
    if (length(cpg_matches) == 0L) {
      next
    }
    cpg_starts_zero_based <- Biostrings::start(cpg_matches) - 1L
    cpg_ends_exclusive <- Biostrings::end(cpg_matches)
    output_lines <- paste(
      sequence_name,
      cpg_starts_zero_based,
      cpg_ends_exclusive,
      "+",
      sep = "\t"
    )
    writeLines(output_lines, output_connection)
    total_cpg_count <- total_cpg_count + length(cpg_matches)
  }

  list(
    cpg_count = total_cpg_count,
    selected_contigs = selected_names,
    sequence_count = length(sequences)
  )
}

read_ron_cpgs <- function(path, output_path) {
  input_connection <- file(path, open = "rt")
  on.exit(close(input_connection), add = TRUE)
  output_connection <- file(output_path, open = "wt")
  on.exit(close(output_connection), add = TRUE)

  current_chr <- NULL
  current_start <- NULL
  current_end <- NULL
  current_strand <- NULL
  cpg_count <- 0L

  repeat {
    line <- readLines(input_connection, n = 1L)
    if (length(line) == 0L) {
      break
    }

    chr_match <- regmatches(line, regexec('chr: "([^"]+)"', line, fixed = FALSE))[[1L]]
    if (length(chr_match) > 1L) {
      current_chr <- chr_match[[2L]]
    }
    start_match <- regmatches(line, regexec("start: ([0-9]+)", line, fixed = FALSE))[[1L]]
    if (length(start_match) > 1L) {
      current_start <- as.integer(start_match[[2L]])
    }
    end_match <- regmatches(line, regexec("end: ([0-9]+)", line, fixed = FALSE))[[1L]]
    if (length(end_match) > 1L) {
      current_end <- as.integer(end_match[[2L]])
    }
    strand_match <- regmatches(line, regexec("strand: '([^']+)'", line, fixed = FALSE))[[1L]]
    if (length(strand_match) > 1L) {
      current_strand <- strand_match[[2L]]
    }

    if (!is.null(current_chr) && !is.null(current_start) && !is.null(current_end) && !is.null(current_strand)) {
      writeLines(
        paste(current_chr, current_start, current_end, current_strand, sep = "\t"),
        output_connection
      )
      cpg_count <- cpg_count + 1L
      current_chr <- NULL
      current_start <- NULL
      current_end <- NULL
      current_strand <- NULL
    }
  }

  list(cpg_count = cpg_count)
}

fasta_records_path <- file.path(output_directory, "fasta-cpgs.tsv")
ron_records_path <- file.path(output_directory, "ron-cpgs.tsv")
fasta_summary <- read_fasta_cpgs(fasta_path, fasta_records_path)
ron_summary <- read_ron_cpgs(ron_path, ron_records_path)

write_json <- function(value, path) {
  if (!requireNamespace("jsonlite", quietly = TRUE)) {
    stop("The CRAN package 'jsonlite' is required", call. = FALSE)
  }
  jsonlite::write_json(value, path, auto_unbox = TRUE, pretty = TRUE)
}

result <- list(
  schema_version = "gate6.cpg-universe-compare/v1",
  fasta_path = fasta_path,
  cpg_ron_path = ron_path,
  fasta_sequence_count = fasta_summary$sequence_count,
  fasta_selected_contigs = fasta_summary$selected_contigs,
  fasta_cpg_count = fasta_summary$cpg_count,
  ron_cpg_count = ron_summary$cpg_count,
  fasta_records_path = fasta_records_path,
  ron_records_path = ron_records_path,
  coordinate_contract = "both normalized to chr, 0-based start, end-exclusive, strand",
  next_step = "sort both normalized files externally and compare byte identity"
)
write_json(result, file.path(output_directory, "cpg-universe-compare-preflight.json"))
cat(jsonlite::toJSON(result, auto_unbox = TRUE, pretty = TRUE), "\n")
