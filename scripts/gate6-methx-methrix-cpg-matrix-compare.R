#!/usr/bin/env Rscript

arguments <- commandArgs(trailingOnly = TRUE)
if (length(arguments) != 4L) {
  stop(
    "Usage: gate6-methx-methrix-cpg-matrix-compare.R <cov_dir> <methx_h5> <genome> <output_dir>",
    call. = FALSE
  )
}

cov_directory <- normalizePath(arguments[[1L]], mustWork = TRUE)
methx_h5_path <- normalizePath(arguments[[2L]], mustWork = TRUE)
genome_name <- tolower(arguments[[3L]])
output_directory <- normalizePath(arguments[[4L]], mustWork = FALSE)
dir.create(output_directory, recursive = TRUE, showWarnings = FALSE)
temporary_directory <- file.path(output_directory, "methrix-h5-temp")
dir.create(temporary_directory, recursive = TRUE, showWarnings = FALSE)

required_packages <- c(
  "methrix",
  "rhdf5",
  "jsonlite",
  "SummarizedExperiment",
  "GenomicRanges",
  "IRanges"
)
missing_packages <- required_packages[!vapply(required_packages, requireNamespace, logical(1), quietly = TRUE)]
if (length(missing_packages) > 0L) {
  stop(
    sprintf("Missing required R packages: %s", paste(missing_packages, collapse = ", ")),
    call. = FALSE
  )
}

reference_source <- "Methx assays.h5 rowData CpG universe"

assert_true <- function(condition, message) {
  if (!isTRUE(condition)) {
    stop(message, call. = FALSE)
  }
}

read_hdf5_dataset <- function(path, dataset_path) {
  tryCatch(
    rhdf5::h5read(path, dataset_path),
    error = function(error) {
      stop(
        sprintf("Unable to read %s from %s: %s", dataset_path, path, conditionMessage(error)),
        call. = FALSE
      )
    }
  )
}

normalize_assay <- function(values, cpg_count, sample_count, assay_name) {
  dimensions <- as.integer(dim(values))
  if (identical(dimensions, c(cpg_count, sample_count))) {
    return(values)
  }
  if (identical(dimensions, c(sample_count, cpg_count))) {
    return(t(values))
  }
  stop(
    sprintf(
      "%s dimensions %s do not match %d CpGs x %d samples",
      assay_name,
      paste(dimensions, collapse = " x "),
      cpg_count,
      sample_count
    ),
    call. = FALSE
  )
}

extract_coordinate_data <- function(methrix_object) {
  coordinate_data <- as.data.frame(SummarizedExperiment::rowData(methrix_object), stringsAsFactors = FALSE)
  required_columns <- c("chr", "start", "end", "strand")
  if (all(required_columns %in% names(coordinate_data))) {
    return(coordinate_data[, required_columns, drop = FALSE])
  }

  if (all(c("chr", "start", "strand") %in% names(coordinate_data))) {
    coordinate_data$end <- as.integer(coordinate_data$start) + 1L
    return(coordinate_data[, required_columns, drop = FALSE])
  }

  if (all(c("seqnames", "start", "strand") %in% names(coordinate_data))) {
    coordinate_data$chr <- as.character(coordinate_data$seqnames)
    coordinate_data$end <- as.integer(coordinate_data$start) + 1L
    return(coordinate_data[, required_columns, drop = FALSE])
  }

  methrix_ranges <- tryCatch(
    SummarizedExperiment::rowRanges(methrix_object),
    error = function(error) NULL
  )
  if (!is.null(methrix_ranges)) {
    return(data.frame(
      chr = as.character(GenomicRanges::seqnames(methrix_ranges)),
      start = as.integer(GenomicRanges::start(methrix_ranges)),
      end = as.integer(GenomicRanges::end(methrix_ranges)),
      strand = as.character(GenomicRanges::strand(methrix_ranges)),
      stringsAsFactors = FALSE
    ))
  }

  stop(
    sprintf("Methrix object has no recognized coordinate columns; found: %s", paste(names(coordinate_data), collapse = ", ")),
    call. = FALSE
  )
}

calculate_difference_summary <- function(methx_values, methrix_values, tolerance) {
  methx_is_missing <- is.na(methx_values) | is.nan(methx_values)
  methrix_is_missing <- is.na(methrix_values) | is.nan(methrix_values)
  missingness_mismatch <- methx_is_missing != methrix_is_missing
  comparable_values <- !methx_is_missing & !methrix_is_missing
  absolute_difference <- abs(methx_values[comparable_values] - methrix_values[comparable_values])
  list(
    missingness_mismatch_count = sum(missingness_mismatch),
    comparable_count = sum(comparable_values),
    value_mismatch_count = sum(absolute_difference > tolerance),
    maximum_absolute_difference = if (length(absolute_difference) == 0L) 0 else max(absolute_difference),
    mean_absolute_difference = if (length(absolute_difference) == 0L) 0 else mean(absolute_difference),
    tolerance = tolerance
  )
}

write_json <- function(value, path) {
  jsonlite::write_json(value, path, auto_unbox = TRUE, pretty = TRUE, na = "null")
}

cov_files <- list.files(
  cov_directory,
  pattern = "\\.bismark\\.cov(?:\\.gz)?$",
  full.names = TRUE
)
cov_files <- sort(cov_files)
assert_true(length(cov_files) > 0L, sprintf("No Bismark coverage files found in %s", cov_directory))

methx_sequence_names <- as.character(read_hdf5_dataset(methx_h5_path, "/rowData/seqnames"))
methx_starts <- as.integer(read_hdf5_dataset(methx_h5_path, "/rowData/start"))
methx_ends <- as.integer(read_hdf5_dataset(methx_h5_path, "/rowData/end"))
methx_strands <- as.character(read_hdf5_dataset(methx_h5_path, "/rowData/strand"))
methx_sample_names <- as.character(read_hdf5_dataset(methx_h5_path, "/colData/sample_name"))
methx_cpg_count <- length(methx_sequence_names)
methx_sample_count <- length(methx_sample_names)
methx_beta <- normalize_assay(
  read_hdf5_dataset(methx_h5_path, "/beta"),
  methx_cpg_count,
  methx_sample_count,
  "Methx beta"
)
methx_coverage <- normalize_assay(
  read_hdf5_dataset(methx_h5_path, "/cov"),
  methx_cpg_count,
  methx_sample_count,
  "Methx coverage"
)

reference_cpgs <- list(
  cpgs = data.table::data.table(
    chr = methx_sequence_names,
    start = methx_starts,
    end = methx_ends,
    width = methx_ends - methx_starts,
    strand = methx_strands
  ),
  contig_lens = data.table::data.table(
    contig = unique(methx_sequence_names),
    length = NA_integer_
  ),
  release_name = genome_name
)
methrix_object <- methrix::read_bedgraphs(
  files = cov_files,
  ref_cpgs = reference_cpgs,
  chr_idx = 2,
  start_idx = 3,
  M_idx = 5,
  U_idx = 6,
  stranded = TRUE,
  collapse_strands = TRUE,
  pipeline = "Bismark_cov",
  zero_based = FALSE,
  n_threads = 4,
  h5 = FALSE,
  h5_dir = NULL,
  h5temp = NULL,
  vect = FALSE,
  verbose = FALSE
)
methrix_clean <- methrix_object
methrix_beta <- as.matrix(methrix::get_matrix(methrix_clean, type = "M", add_loci = FALSE))
methrix_coverage <- as.matrix(methrix::get_matrix(methrix_clean, type = "C", add_loci = FALSE))
methrix_coordinates <- extract_coordinate_data(methrix_clean)
methrix_sample_names <- colnames(methrix_beta)

methx_sequence_names <- as.character(read_hdf5_dataset(methx_h5_path, "/rowData/seqnames"))
methx_starts <- as.integer(read_hdf5_dataset(methx_h5_path, "/rowData/start"))
methx_ends <- as.integer(read_hdf5_dataset(methx_h5_path, "/rowData/end"))
methx_strands <- as.character(read_hdf5_dataset(methx_h5_path, "/rowData/strand"))
methx_sample_names <- as.character(read_hdf5_dataset(methx_h5_path, "/colData/sample_name"))
methx_cpg_count <- length(methx_sequence_names)
methx_sample_count <- length(methx_sample_names)
methx_beta <- normalize_assay(
  read_hdf5_dataset(methx_h5_path, "/beta"),
  methx_cpg_count,
  methx_sample_count,
  "Methx beta"
)
methx_coverage <- normalize_assay(
  read_hdf5_dataset(methx_h5_path, "/cov"),
  methx_cpg_count,
  methx_sample_count,
  "Methx coverage"
)

assert_true(nrow(methrix_beta) == nrow(methrix_coordinates), "Methrix beta and coordinate row counts differ")
assert_true(nrow(methrix_beta) == nrow(methrix_coverage), "Methrix beta and coverage row counts differ")
assert_true(ncol(methrix_beta) == length(methrix_sample_names), "Methrix beta and sample name counts differ")
assert_true(nrow(methrix_beta) == methx_cpg_count, "Methx and Methrix CpG row counts differ")
assert_true(ncol(methrix_beta) == methx_sample_count, "Methx and Methrix sample counts differ")

methrix_locus_strands <- methrix_coordinates$strand
methrix_locus_strands[methrix_locus_strands == "*"] <- "+"
methrix_locus_ids <- sprintf(
  "%s|%d|%d|%s",
  methrix_coordinates$chr,
  as.integer(methrix_coordinates$start),
  as.integer(methrix_coordinates$end),
  methrix_locus_strands
)
methx_locus_ids <- sprintf(
  "%s|%d|%d|%s",
  methx_sequence_names,
  methx_starts,
  methx_ends,
  methx_strands
)
coordinate_match <- identical(methrix_locus_ids, methx_locus_ids)
coordinate_set_match <- length(methrix_locus_ids) == length(methx_locus_ids) &&
  !anyDuplicated(methrix_locus_ids) &&
  !anyDuplicated(methx_locus_ids) &&
  setequal(methrix_locus_ids, methx_locus_ids)
coordinate_intersection_count <- length(intersect(methrix_locus_ids, methx_locus_ids))
methrix_only_loci <- setdiff(methrix_locus_ids, methx_locus_ids)
methx_only_loci <- setdiff(methx_locus_ids, methrix_locus_ids)
if (coordinate_set_match && !coordinate_match) {
  methrix_reordering <- match(methx_locus_ids, methrix_locus_ids)
  methrix_beta <- methrix_beta[methrix_reordering, , drop = FALSE]
  methrix_coverage <- methrix_coverage[methrix_reordering, , drop = FALSE]
  methrix_coordinates <- methrix_coordinates[methrix_reordering, , drop = FALSE]
  methrix_locus_ids <- methrix_locus_ids[methrix_reordering]
  coordinate_match <- identical(methrix_locus_ids, methx_locus_ids)
}
sample_name_match <- identical(methrix_sample_names, methx_sample_names)

coverage_difference <- calculate_difference_summary(
  methx_coverage,
  methrix_coverage,
  tolerance = 0
)
beta_difference <- calculate_difference_summary(
  methx_beta,
  methrix_beta,
  tolerance = 1e-6
)

mismatch_rows <- which(
  (is.na(methx_coverage) != is.na(methrix_coverage)) |
    (methx_coverage != methrix_coverage) |
    (is.na(methx_beta) != is.na(methrix_beta)) |
    (!is.na(methx_beta) & !is.na(methrix_beta) & abs(methx_beta - methrix_beta) > 1e-6),
  arr.ind = TRUE
)
mismatch_preview_path <- file.path(output_directory, "matrix-mismatch-preview.tsv")
if (nrow(mismatch_rows) > 0L) {
  preview_rows <- mismatch_rows[seq_len(min(nrow(mismatch_rows), 100L)), , drop = FALSE]
  preview <- data.frame(
    cpg_index = preview_rows[, 1],
    sample_index = preview_rows[, 2],
    locus = methx_locus_ids[preview_rows[, 1]],
    sample_name = methx_sample_names[preview_rows[, 2]],
    methx_beta = methx_beta[preview_rows],
    methrix_beta = methrix_beta[preview_rows],
    methx_coverage = methx_coverage[preview_rows],
    methrix_coverage = methrix_coverage[preview_rows],
    stringsAsFactors = FALSE
  )
  utils::write.table(preview, mismatch_preview_path, sep = "\t", quote = FALSE, row.names = FALSE, na = "NA")
} else {
  utils::write.table(
    data.frame(status = "no_mismatches"),
    mismatch_preview_path,
    sep = "\t",
    quote = FALSE,
    row.names = FALSE
  )
}

result <- list(
  schema_version = "gate6.methx-methrix-cpg-matrix/v1",
  genome = genome_name,
  reference_source = reference_source,
  methrix_version = as.character(utils::packageVersion("methrix")),
  cov_directory = cov_directory,
  cov_files = cov_files,
  methx_h5 = methx_h5_path,
  methrix_output_directory = output_directory,
  cpg_count = methx_cpg_count,
  sample_count = methx_sample_count,
  sample_names = methx_sample_names,
  coordinate_match = coordinate_match,
  coordinate_set_match = coordinate_set_match,
  coordinate_intersection_count = coordinate_intersection_count,
  methrix_only_locus_count = length(methrix_only_loci),
  methx_only_locus_count = length(methx_only_loci),
  methrix_first_loci = head(methrix_locus_ids, 5L),
  methx_first_loci = head(methx_locus_ids, 5L),
  methrix_only_loci_preview = head(methrix_only_loci, 5L),
  methx_only_loci_preview = head(methx_only_loci, 5L),
  sample_name_match = sample_name_match,
  coverage = coverage_difference,
  beta = beta_difference,
  matrix_mismatch_cell_count = nrow(mismatch_rows),
  matrix_mismatch_preview = mismatch_preview_path,
  pass = isTRUE(coordinate_match) && isTRUE(sample_name_match) &&
    coverage_difference$missingness_mismatch_count == 0L &&
    coverage_difference$value_mismatch_count == 0L &&
    beta_difference$missingness_mismatch_count == 0L &&
    beta_difference$value_mismatch_count == 0L
)
write_json(result, file.path(output_directory, "matrix-comparison.json"))

cat(jsonlite::toJSON(result, auto_unbox = TRUE, pretty = TRUE, na = "null"))
cat("\n")
if (!isTRUE(result$pass)) {
  quit(save = "no", status = 2L, runLast = FALSE)
}
