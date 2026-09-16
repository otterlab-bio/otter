library(yaml)
library(glue)
library(dplyr)
library(stringr)
library(data.table)

library(optparse)
option_list = list(make_option(c("--configfolder"), type = "character", default = NULL, help = "file, provided by default."),
                   make_option(c("--output_dir"), type = "character", default = NULL, help = "_human.txt, provided by default."))
args = commandArgs(trailingOnly=F)
args <- parse_args(OptionParser(option_list = option_list))
# 获取命令行参数

QC_summary <- function(configfile){

  config <- yaml::read_yaml(configfile)
  SIDs <- config$SIDs
  qc_df_list <- list()
  graft <- config$workflow$species$graft
  for (i in 1:length(SIDs)){
    seqkit_file <- glue::glue("{config$directories$qc$main}/{SIDs[i]}_seqkit_stat.txt")
    message(seqkit_file)
    # seqkit
    seqStatT = read.table(seqkit_file, header = TRUE)
    reads_raw_R1 = seqStatT[1, "num_seqs"]
    bases_raw_R1 = seqStatT[1, "sum_len"]
    Q20_raw_read1 = seqStatT[1, "Q20..."]
    Q30_raw_read1 = seqStatT[1, "Q30..."]
    min_len_raw_read1 = seqStatT[1, "min_len"]
    avg_len_raw_read1 = seqStatT[1, "avg_len"]
    max_len_raw_read1 = seqStatT[1, "max_len"]

    reads_raw_R2 = seqStatT[2, "num_seqs"]
    bases_raw_R2 = seqStatT[2, "sum_len"]
    Q20_raw_read2 = seqStatT[2, "Q20..."]
    Q30_raw_read2 = seqStatT[2, "Q30..."]
    min_len_raw_read2 = seqStatT[2, "min_len"]
    avg_len_raw_read2 = seqStatT[2, "avg_len"]
    max_len_raw_read2 = seqStatT[2, "max_len"]

    reads_clean_R1 = seqStatT[3, "num_seqs"]
    bases_clean_R1 = seqStatT[3, "sum_len"]
    Q20_clean_read1 = seqStatT[3, "Q20..."]
    Q30_clean_read1 = seqStatT[3, "Q30..."]
    min_len_clean_read1 = seqStatT[3, "min_len"]
    avg_len_clean_read1 = seqStatT[3, "avg_len"]
    max_len_clean_read1 = seqStatT[3, "max_len"]

    reads_clean_R2 = seqStatT[4, "num_seqs"]
    bases_clean_R2 = seqStatT[4, "sum_len"]
    Q20_clean_read2 = seqStatT[4, "Q20..."]
    Q30_clean_read2 = seqStatT[4, "Q30..."]
    min_len_clean_read2 = seqStatT[4, "min_len"]
    avg_len_clean_read2 = seqStatT[4, "avg_len"]
    max_len_clean_read2 = seqStatT[4, "max_len"]

    reads_raw = reads_raw_R1 + reads_raw_R2
    bases_raw = bases_raw_R1 + bases_raw_R2
    reads_clean = reads_clean_R1 + reads_clean_R2
    bases_clean = bases_clean_R1 + bases_clean_R2
    clean_data_ratio = signif(bases_clean / bases_raw, 4)

    # trim
    trimQcR1 = glue::glue("{config$output$trim_dir}/{SIDs[i]}_R1.fastq.gz_trimming_report.txt")
    message(trimQcR1)
    trimQcR1T = readLines(trimQcR1)
    reads_with_adapter_R1 = trimQcR1T[grep("Reads with adapters:  ",trimQcR1T)] %>%
      stringr::str_remove(.,"Reads with adapters:  ") %>%
      stringr::str_trim()
    reads_write_R1 = trimQcR1T[grep("Reads written",trimQcR1T)] %>%
      stringr::str_split(.,":") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()
    bp_qc_remove_R1 = trimQcR1T[grep("Quality-trimmed:",trimQcR1T)] %>%
      stringr::str_split(.,":") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()

    bp_write_R1 = trimQcR1T[grep("Total written",trimQcR1T)] %>%
      stringr::str_split(.,":") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()
    trimQcR2 = glue::glue("{config$output$trim_dir}/{SIDs[i]}_R2.fastq.gz_trimming_report.txt")
    trimQcR2T = readLines(trimQcR2)
    reads_with_adapter_R2 = trimQcR2T[grep("Reads with adapters:  ",trimQcR2T)] %>%
      stringr::str_remove(.,"Reads with adapters:  ") %>%
      stringr::str_trim()

    reads_write_R2 = trimQcR2T[grep("Reads written",trimQcR2T)] %>%
      stringr::str_split(.,":") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()

    bp_qc_remove_R2 = trimQcR2T[grep("Quality-trimmed:",trimQcR2T)] %>%
      stringr::str_split(.,":") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()

    bp_write_R2 = trimQcR2T[grep("Total written",trimQcR2T)] %>%
      stringr::str_split(.,":") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()
    # GC_raw
    GC_R1_raw = glue::glue("{config$directories$qc$before}/{SIDs[i]}_R1_fastqc/fastqc_data.txt")
    GC_R2_raw = glue::glue("{config$directories$qc$before}/{SIDs[i]}_R2_fastqc/fastqc_data.txt")
    GC_R1 <- readLines(GC_R1_raw)[11] %>%
      stringr::str_split(.,"\t") %>%
      {as.numeric(.[[1]][2])}
    GC_R2 <- readLines(GC_R2_raw)[11] %>%
      stringr::str_split(.,"\t") %>%
      {as.numeric(.[[1]][2])}
    GC_raw = (GC_R1+GC_R2)/2

    # GC_clean

    GC_R1_clean = glue::glue("{config$directories$qc$after}/{SIDs[i]}_val_1_fastqc/fastqc_data.txt")
    GC_R2_clean = glue::glue("{config$directories$qc$after}/{SIDs[i]}_val_2_fastqc/fastqc_data.txt")
    GC_R1 <- readLines(GC_R1_clean)[11] %>%
      stringr::str_split(.,"\t") %>%
      {as.numeric(.[[1]][2])}
    GC_R2 <- readLines(GC_R2_clean)[11] %>%
      stringr::str_split(.,"\t") %>%
      {as.numeric(.[[1]][2])}
    GC_clean = (GC_R1+GC_R2)/2

    # bismark/STAR
    bismarkfile = glue::glue("{config$directories$bsmap$main}/{graft}/{SIDs[i]}Log.final.out")
    bsmapStatT = readLines(bismarkfile)
    mapping_ratio = bsmapStatT[grep("Uniquely mapped reads % ",bsmapStatT)] %>%
      stringr::str_split(.,"\t") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()
    total_reads_pairs = bsmapStatT[grep("Number of input reads",bsmapStatT)] %>%
      stringr::str_split(.,"\t") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()
    aligned_reads_pairs = bsmapStatT[grep("Uniquely mapped reads number",bsmapStatT)] %>%
      stringr::str_split(.,"\t") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()
    aligned_reads_paires_ratio = as.numeric(aligned_reads_pairs)/as.numeric(total_reads_pairs)
    unique_reads_pairs = aligned_reads_pairs
    unique_reads_pairs_ratio = aligned_reads_paires_ratio

    # qualimap
    qualimapfile = glue::glue("{config$directories$qc$main}/qualimap/{SIDs[i]}_{graft}/genome_results.txt")
    message(qualimapfile)
    qualimapT = readLines(qualimapfile)
    mapping_quality = qualimapT[grep("mean mapping quality =",qualimapT)] %>%
      stringr::str_split(.,"=") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()
    duplicated_reads = qualimapT[grep("number of duplicated reads",qualimapT)] %>%
      stringr::str_split(.,"=") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()
    duplication_ratio = qualimapT[grep("duplication rate =",qualimapT)] %>%
      stringr::str_split(.,"=") %>%
      {.[[1]][2]} %>%
      stringr::str_trim()
    # summary
    data_volume_raw = signif(bases_raw / 10^9, 6)
    data_volume_clean = signif(bases_clean / 10^9, 6)
    genome_reference = "hg19"

    qc_df_list[[i]] <- data.frame(
      sampleid = SIDs[i],
      reads_raw = reads_raw,
      bases_raw = bases_raw,
      reads_clean = reads_clean,
      bases_clean = bases_clean,
      Q20_raw_read1= Q20_raw_read1,
      Q30_raw_read1= Q30_raw_read1,
      min_len_raw_read1 = min_len_raw_read1 ,
      avg_len_raw_read1 = avg_len_raw_read1,
      max_len_raw_read1= max_len_raw_read1,
      Q20_raw_read2=Q20_raw_read2,
      Q30_raw_read2=Q30_raw_read2,
      min_len_raw_read2=min_len_raw_read2,
      avg_len_raw_read2=avg_len_raw_read2,
      max_len_raw_read2=max_len_raw_read2,

      Q20_clean_read1= Q20_clean_read1,
      Q30_clean_read1= Q30_clean_read1,
      min_len_clean_read1 = min_len_clean_read1 ,
      avg_len_clean_read1 = avg_len_clean_read1,
      max_len_clean_read1= max_len_clean_read1,
      Q20_clean_read2=Q20_clean_read2,
      Q30_clean_read2=Q30_clean_read2,
      min_len_clean_read2=min_len_clean_read2,
      avg_len_clean_read2=avg_len_clean_read2,
      max_len_clean_read2=max_len_clean_read2,

      clean_data_ratio = clean_data_ratio,
      reads_with_adapter_R1 = reads_with_adapter_R1,
      reads_write_R1 = reads_write_R1,
      bp_qc_remove_R1 = bp_qc_remove_R1,
      bp_write_R1 = bp_write_R1,
      reads_with_adapter_R2 = reads_with_adapter_R2,
      reads_write_R2 = reads_write_R2,
      bp_qc_remove_R2 = bp_qc_remove_R2,
      bp_write_R2 = bp_write_R2,
      GC_raw = GC_raw,
      GC_clean = GC_clean,
      mapping_ratio = mapping_ratio,
      total_reads_pairs = total_reads_pairs,
      aligned_reads_pairs = aligned_reads_pairs,
      aligned_reads_paires_ratio = aligned_reads_paires_ratio,
      unique_reads_pairs = unique_reads_pairs,
      unique_reads_pairs_ratio = unique_reads_pairs_ratio,
      mapping_quality = mapping_quality,
      duplicated_reads = duplicated_reads,
      duplication_ratio = duplication_ratio,
      data_volume_raw = data_volume_raw,
      data_volume_clean = data_volume_clean,
      genome_reference = genome_reference
    )
  }
  qc_df <- data.table::rbindlist(qc_df_list) %>%
    as.data.frame()
  if (file.exists(paste0(config$directories$bsmap$main,"/Filtered_bams/filtered_reads_summary.xlsx"))){
    filtered_patch_qc <- openxlsx::read.xlsx(paste0(config$directories$bsmap$main,
                                                    "/Filtered_bams/filtered_reads_summary.xlsx"))
    qc_df <- qc_df %>%
      dplyr::left_join(filtered_patch_qc,
                       by = "sampleid")
  }
  return(qc_df)
}

configfolder <- args$configfolder
output_dir <- args$output_dir
configfile <- paste0(configfolder,"/config.yaml")
QC_summary <- QC_summary(configfile)



QC_summary %>%
  write.table(.,paste0(output_dir,"/qc_summary.txt"),row.names = F,quote = F,sep = " ")

QC_summary %>%
  openxlsx::write.xlsx(.,
                       paste0(output_dir,"/qc_summary.xlsx"))

QC_summary %>%
  saveRDS(.,paste0(output_dir,"/qc_summary.RDS"))
