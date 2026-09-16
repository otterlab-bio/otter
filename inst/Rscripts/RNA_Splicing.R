#R/RNA_Splicing.R
library(optparse)
library(dplyr)
library(stringr)
option_list = list(make_option(c("--root"),
                               type = "character",
                               default = NULL,
                               help = "root dir, provided by default."),
                   make_option(c("--threads"),
                               type = "character",
                               default = "10",
                               help = "cores, provided by default."),
                   make_option(c("--pdata"),
                               type = "character",
                               default = NULL,
                               help = "pdata, provided by default."),
                   make_option(c("--seqlengthQC"),
                               type = "character",
                               default = NULL,
                               help = "seqlengthQC, provided by default."),
                   #gtf
                   make_option(c("--gtf"),
                               type = "character",
                               default = NULL,
                               help = "gtf, provided by default."),
                   #pdxmode
                   make_option(c("--pdxmode"),
                               type = "character",
                               default = NULL,
                               help = "pdxmode, provided by default.")
                   )
args = commandArgs(trailingOnly=F)
args <- parse_args(OptionParser(option_list = option_list))
# 获取命令行参数
root_dir <- args$root

cores <- args$threads %>%
  as.numeric()
pdata <- args$pdata
seqlengthQC <- args$seqlengthQC


# readin pdata
pdata <- openxlsx::read.xlsx(xlsxFile = pdata)

#扫描root_dir
bamfile_dir <- root_dir
bamfile_pattern <- "*.bam$"

if (args$pdxmode == "1"){
  bamfile_dir <- paste0(root_dir,
                        "/Filtered_bams")
  bamfile_pattern <- "*_Filtered.bam$"
}


bam_files <- list.files(path = bamfile_dir,
                        pattern = bamfile_pattern,
                        full.names = F)

bam_files_clean <- bam_files %>%
  stringr::str_remove(.,"_Filtered.bam") %>%
  stringr::str_remove(.,"_fixed")
# make tbls
bam_tbl <- data.frame(
  bams = bam_files_clean
) %>%
  dplyr::mutate(
    bam_dir = bam_files
  ) %>%
  tidyr::separate(.,
                  col = "bams",
                  into = c("sampleid","type"),
                  sep = "_",
                  remove = T) %>%
  dplyr::left_join(pdata,by = "sampleid")

type_levels <- levels(factor(bam_tbl[["type"]]))
type_length <- length(type_levels)

# 获取seqkit的length统计参数
# 计算去除adapter后的read length

seq_length_files <- list.files(path = seqlengthQC,
                               pattern = "*_seqkit_stat.txt$",
                               full.names = T)
seq_tbl <- NULL
for (i in 1:length(seq_length_files)){
  temp_length <- readr::read_delim(seq_length_files[i],
                                   delim = "\t",
                                   escape_double = FALSE,
                                   trim_ws = TRUE) %>%
    dplyr::select(
      all_of(c("file","N50"))
    ) %>%
    dplyr::filter(grepl("_val_",file))

  if (is.null(seq_tbl)){
    seq_tbl <- temp_length
  }else{
    seq_tbl <- seq_tbl %>%
      rbind(.,temp_length)
  }
}

seq_length <- round(
  mean(as.numeric(seq_tbl[["N50"]]) ,
       na.rm = T)
)

# 获取所有对比组合
group_names <- levels(factor(bam_tbl[["sample_group"]]))
combinations <- combn(group_names,
                      2,
                      simplify = FALSE)
# 按types内进行组合间计算
for (i in 1:type_length){
  message(glue::glue(
    ">> processing {type_levels[i]}"
  ))
  bam_tbl_level <- bam_tbl %>%
    dplyr::filter(type %in% type_levels[i])
  for (a in length(combinations)){
    task_title <- paste(combinations[[a]],collapse = '_vs_')
    message(glue::glue(
      ">> make contrast in group {task_title}"
      ))
    b1_files <- bam_tbl_level %>%
      dplyr::filter(sample_group %in% combinations[[a]][1]) %>%
      pull(bam_dir)
    b2_files <- bam_tbl_level %>%
      dplyr::filter(sample_group %in% combinations[[a]][2]) %>%
      pull(bam_dir)
    runtime_dir <- glue::glue("{root_dir}/RNASplicing/{task_title}")
    runtime_tempdir <- glue::glue("{root_dir}/RNASplicing/{task_title}/temp")
    fs::dir_create(path = runtime_dir)
    fs::dir_create(path = runtime_tempdir)
    paste0(b1_files,collapse = ",") %>%
      writeLines(.,
                 con = glue::glue("{runtime_dir}/b1.txt") )

    paste0(b2_files,collapse = ",") %>%
      writeLines(.,
                 con = glue::glue("{runtime_dir}/b2.txt") )
    command <- glue::glue(
      "enva run otter-core -- rmats.py --b1 {runtime_dir}/b1.txt --b2 {runtime_dir}/b2.txt --gtf {args$gtf} -t paired --readLength {seq_length} --variable-read-length --nthread {cores} --od {runtime_dir} --tmp {runtime_tempdir}"
      )
    status <- system(command = command,
                     intern = T)
    message(status)
  }

}
